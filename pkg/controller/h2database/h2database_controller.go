package h2database

import (
	"context"
	"reflect"

	h2v1alpha1 "github.com/pwegrzyn/kubernetes-operators-project/pkg/apis/h2/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/client-go/tools/clientcmd"
	"bytes"
	"fmt"
	corev1client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/kubernetes/scheme"
)

var log = logf.Log.WithName("controller_h2database")

// INFO: This is the logic of the controller, we need to provide it.

// Add creates a new H2Database Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileH2Database{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("h2database-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource H2Database
	err = c.Watch(&source.Kind{Type: &h2v1alpha1.H2Database{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &h2v1alpha1.H2Database{},
	})

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &h2v1alpha1.H2Database{},
	})

	if err != nil {
		return err
	}
	

	return nil
}

// blank assignment to verify that ReconcileH2Database implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileH2Database{}

// ReconcileH2Database reconciles a H2Database object
type ReconcileH2Database struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a H2Database object and makes changes based on the state read
// and what is in the H2Database.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// ***************************************************************************
// Currently this Reconcile loop does the following thigs:
// Create a H2 Deployment if it doesn't exist
// Update the H2 CR status with the names of the H2 pods
// Ensure that the Deployment size is the same as specified by the H2 CR spec
func (r *ReconcileH2Database) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling H2Database")

	// Fetch the H2Database instance
	instance := &h2v1alpha1.H2Database{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("H2Database resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get H2Database.")
		return reconcile.Result{}, err
	}


	// Check if the Deployment already exists, if not create a new one
	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Deployment
		dep := r.deploymentForH2Database(instance)
		reqLogger.Info("Creating a new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue
		// NOTE: that the requeue is made with the purpose to provide the deployment object for the next step to ensure the deployment size is the same as the spec.
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment.")
		return reconcile.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := instance.Spec.Size
	if *deployment.Spec.Replicas != size {
		deployment.Spec.Replicas = &size
		err = r.client.Update(context.TODO(), deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return reconcile.Result{}, err
		}
	}

	// Check if the Service already exists, if not create a new one
	// NOTE: The Service is used to expose the Deployment.
	service := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service object
		ser := r.serviceForH2Database(instance)
		reqLogger.Info("Creating a new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
		err = r.client.Create(context.TODO(), ser)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service.")
		return reconcile.Result{}, err
	}

	// Update the H2DB status with the pod names
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelsForH2Database(instance.Name)),
	}
	err = r.client.List(context.TODO(), podList, listOpts...)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods.", "H2Database.Namespace", instance.Namespace, "H2Database.Name", instance.Name)
		return reconcile.Result{}, err
	}
	// List the pods for this memcached's deployment
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, instance.Status.Nodes) {
		instance.Status.Nodes = podNames
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update H2Database status.")
			return reconcile.Result{}, err
		}
	}

	// Backup the H2 data to a remote location
	dataBackup := instance.Spec.Backup
	if dataBackup != "skip" && len(podList.Items) > 0 {
		// Actually execute the backup inside one of the pods
		backupLocation := "/tmp/h2_backup.zip"
		h2DataLocation := "/opt/h2-data"
		cmdToExec := fmt.Sprintf("apk add curl zip && zip -r %s %s && curl --data \"@%s\" %s", backupLocation, h2DataLocation, backupLocation, dataBackup)
		_, _, cmdErr := ExecuteRemoteCommand(&podList.Items[0], cmdToExec)
		if cmdErr != nil {
			reqLogger.Error(cmdErr, "There may have been an issue with creating the backup...")
		}
		// Skip later checks since this is only a one-time backup feature
		instance.Spec.Backup = "skip"
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update H2Database spec.")
			return reconcile.Result{}, err
		}
	} else if dataBackup == "skip" && len(podList.Items) > 0 {
		reqLogger.Info("Skipping backup.")
	}

	// Use H2's CreateCluster script to make a H2 cluster if the there exactly 2 DB instances
	cluseringMode := instance.Spec.Clustering
	if cluseringMode == "yes" && len(podList.Items) > 0 && size == 2 {
		pod1IP := podList.Items[0].Status.PodIP
		pod2IP := podList.Items[1].Status.PodIP
		// I'm not sure if the ip addreses are properly handled, since both of the addreses will be
		// hidden behind a service
		clusterCmd := fmt.Sprintf("java -cp /opt/h2/bin/h2*.jar org.h2.tools.CreateCluster -urlSource jdbc:h2:tcp://%s:1521/~/test " +
		"-urlTarget jdbc:h2:tcp://%s:1521/~/test -serverList %s:1521,%s:1521", pod1IP, pod2IP, pod1IP, pod2IP)

		// TODO: make sure that we only need to run the CreateCluster script on one machine, and not on both...
		_, _, cmdErr := ExecuteRemoteCommand(&podList.Items[0], clusterCmd)
		if cmdErr != nil {
			reqLogger.Error(cmdErr, "There may have been an issue with starting Cluster Mode..")
		}
		// Change the state to 'issued' to indicate that the mode has been switched to ClusterMode
		instance.Spec.Clustering = "issued"
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update H2Database spec.")
			return reconcile.Result{}, err
		}
	} else if cluseringMode == "issued" && len(podList.Items) > 0 {
		reqLogger.Info("Cluster Mode for H2 is issued.")
	} else if cluseringMode == "no" && len(podList.Items) > 0 {
		reqLogger.Info("Skipping Cluter Mode for H2.")
	} else if cluseringMode == "yes" && len(podList.Items) > 0 && size != 2 {
		reqLogger.Info("Cannot run ClusterMode if there is more or less than 2 H2 instances running!")
	}

	return reconcile.Result{}, nil

	// ***********************************************************************

	// TODO: Below is leftover from the default operator-sdk controller

	// // Define a new Pod object
	// pod := newPodForCR(instance)

	// // Set H2Database instance as the owner and controller
	// if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
	// 	return reconcile.Result{}, err
	// }


	// // Check if this Pod already exists
	// found := &corev1.Pod{}
	// err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	// if err != nil && errors.IsNotFound(err) {
	// 	reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
	// 	err = r.client.Create(context.TODO(), pod)
	// 	if err != nil {
	// 		return reconcile.Result{}, err
	// 	}

	// 	// Pod created successfully - don't requeue
	// 	return reconcile.Result{}, nil
	// } else if err != nil {
	// 	return reconcile.Result{}, err
	// }

	// // Pod already exists - don't requeue
	// reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	// return reconcile.Result{}, nil
}

// ExecuteRemoteCommand executes a remote shell command on the given pod
// returns the output from stdout and stderr
func ExecuteRemoteCommand(pod *corev1.Pod, command string) (string, string, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return "", "", err
	}
	coreClient, err := corev1client.NewForConfig(restCfg)
	if err != nil {
		return "", "", err
	}

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := coreClient.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/sh", "-c", command},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restCfg, "POST", request.URL())
	if err != nil {
		return "", "", err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", err
	}

	return buf.String(), errBuf.String(), nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCRBusyBox(cr *h2v1alpha1.H2Database) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

// deploymentForH2Database returns a H2 Deployment object
func (r *ReconcileH2Database) deploymentForH2Database(h *h2v1alpha1.H2Database) *appsv1.Deployment {
	ls := labelsForH2Database(h.Name)
	replicas := h.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "oscarfonts/h2:alpine",
						Name:    "h2database",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 1521,
							Name:          "h2database",
						}},
					}},
				},
			},
		},
	}
	// Set H2 instance as the owner of the Deployment.
	controllerutil.SetControllerReference(h, dep, r.scheme)
	return dep
}

// serviceForH2Database function takes in a H2Database object and returns a Service for that object.
func (r *ReconcileH2Database) serviceForH2Database(h *h2v1alpha1.H2Database) *corev1.Service {
	ls := labelsForH2Database(h.Name)
	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{
					Port: 1521,
					Name: h.Name,
				},
			},
		},
	}
	// Set Memcached instance as the owner of the Service.
	controllerutil.SetControllerReference(h, ser, r.scheme)
	return ser
}

// labelsForH2Database returns the labels for selecting the resources
// belonging to the given h2 CR name.
func labelsForH2Database(name string) map[string]string {
	return map[string]string{"app": "h2database", "h2database_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}