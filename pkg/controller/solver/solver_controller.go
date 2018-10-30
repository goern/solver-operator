package solver

import (
	"context"
	"log"
	"strconv"

	thothv1alpha1 "github.com/thoth-station/solver-operator/pkg/apis/thoth/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Solver Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSolver{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("solver-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Solver
	err = c.Watch(&source.Kind{Type: &thothv1alpha1.Solver{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Solver
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &thothv1alpha1.Solver{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileSolver{}

// ReconcileSolver reconciles a Solver object
type ReconcileSolver struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Solver object and makes changes based on the state read
// and what is in the Solver.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSolver) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Reconciling Solver %s/%s\n", request.Namespace, request.Name)

	// Fetch the Solver instance
	instance := &thothv1alpha1.Solver{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Printf("Solver.Status.Phase = %s", instance.Status.Phase)
	if instance.Status.Phase == thothv1alpha1.SolverPhaseCompleted {
		return reconcile.Result{}, nil
	}

	// Define a new Pod object
	job := newSolverJob(instance)

	// Set Solver instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, job, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Job already exists
	found := &batchv1.Job{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating a new Job %s/%s\n", job.Namespace, job.Name)
		err = r.client.Create(context.TODO(), job)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Job created successfully - don't requeue
		instance.Status.Phase = thothv1alpha1.SolverPhaseRunning
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			log.Printf("failed to update Solver status: %v", err)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Job already exists - don't requeue
	log.Printf("Skip reconcile: Job %s/%s already exists", found.Namespace, found.Name)
	instance.Status.Phase = thothv1alpha1.SolverPhaseRunning
	instance.Status.Active = 1

	if found.Status.Succeeded == 1 {
		log.Printf("Job %s/%s succeeded! Packages was: %s\n", found.Namespace, found.Name, instance.Spec.Packages)

		if !instance.Spec.KeepJob {
			log.Printf("Deleting Job %s/%s...\n", found.Namespace, found.Name)

			err = r.client.Delete(context.TODO(), job)
			if err != nil {
				return reconcile.Result{}, err
			}

			// FIXME Pods of the Job do not get deleted, why?
		}

		instance.Status.Phase = thothv1alpha1.SolverPhaseCompleted
		instance.Status.Succeeded = 1
		instance.Status.Active = 0
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			log.Printf("failed to update Solver status: %v", err)
			return reconcile.Result{}, err
		}
	}

	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		log.Printf("failed to update Solver status: %v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// newSolverJob returns a busybox pod with the same name/namespace as the cr
func newSolverJob(cr *thothv1alpha1.Solver) *batchv1.Job {
	labels := labelsForSolverJob(cr.Name)
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-job",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			// Optional: Parallelism:,
			// Optional: Completions:,
			// Optional: ActiveDeadlineSeconds:,
			// Optional: Selector:,
			// Optional: ManualSelector:,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{}, // Doesn't seem obligatory(?)...
					Containers: []corev1.Container{
						{
							Name:            "solver",
							Image:           "busybox",
							ImagePullPolicy: corev1.PullPolicy(corev1.PullAlways),
							Command:         []string{"env"},
							Env: []corev1.EnvVar{
								{
									Name:  "THOTH_SOLVER",
									Value: "solver-f27",
								},
								{
									Name:  "THOTH_LOG_SOLVER",
									Value: strconv.FormatBool(true),
								},
								{
									Name:  "THOTH_SOLVER_NO_TRANSITIVE",
									Value: strconv.FormatBool(cr.Spec.IncludeTransitive),
								},
								{
									Name:  "THOTH_SOLVER_PACKAGES",
									Value: cr.Spec.Packages,
								},
								{
									Name:  "THOTH_SOLVER_OUTPUT",
									Value: cr.Spec.Output,
								},
							},
							VolumeMounts: []corev1.VolumeMount{},
						},
					},
					RestartPolicy:    corev1.RestartPolicyNever,
					Volumes:          []corev1.Volume{},
					ImagePullSecrets: []corev1.LocalObjectReference{},
				},
			},
		},
		// Optional: JobStatus:,
	}
}

// labelsForSolverJob returns the labels for selecting the resources
// belonging to the given Solver CR name.
func labelsForSolverJob(name string) map[string]string {
	return map[string]string{"app": "thoth", "component": "solver-f27", "solver": name}
}
