package deployments

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"github.com/Li-Elias/Railclone/internal/models"
)

func Create(clientset *kubernetes.Clientset, deployment *models.Deployment) (int32, error) {
	appName := fmt.Sprintf("deployment-%d-user-%d", deployment.ID, deployment.UserID)

	deploymentObj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName + "-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(deployment.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": appName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  appName + "-deployment",
							Image: deployment.Image,
							Env:   []corev1.EnvVar{},
						},
					},
				},
			},
		},
	}

	if deployment.Volume != 0 {
		volume := fmt.Sprintf("%dGi", deployment.Volume)
		storageClassName := appName + "-storage-class"

		pvObj := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: appName + "-pv",
				Labels: map[string]string{
					"type": "local",
					"app":  appName,
				},
			},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: storageClassName,
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(volume),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: fmt.Sprintf("/mnt/%s/data", appName),
					},
				},
			},
		}

		pvcObj := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: appName + "-pv-claim",
				Labels: map[string]string{
					"app": appName,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(volume),
					},
				},
			},
		}

		deploymentObj.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deploymentObj.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      appName + "-volume",
				MountPath: "/var/lib/" + appName + "-volume" + "/data",
			},
		)

		deploymentObj.Spec.Template.Spec.Volumes = append(
			deploymentObj.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: appName + "-volume",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: appName + "-pv-claim",
					},
				},
			},
		)

		_, err := clientset.CoreV1().PersistentVolumes().Create(context.TODO(), pvObj, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}

		_, err = clientset.CoreV1().PersistentVolumeClaims(corev1.NamespaceDefault).Create(context.TODO(), pvcObj, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}
	}

	if len(deployment.EnvVars) != 0 {
		for key, value := range deployment.EnvVars {
			deploymentObj.Spec.Template.Spec.Containers[0].Env = append(
				deploymentObj.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{Name: key, Value: value},
			)
		}
	}

	serviceObj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName + "-service",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": appName,
			},
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port:       5432,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(5432),
				},
			},
		},
	}

	_, err := clientset.AppsV1().Deployments(corev1.NamespaceDefault).Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	if err != nil {
		return 0, err
	}

	createdService, err := clientset.CoreV1().Services(corev1.NamespaceDefault).Create(context.TODO(), serviceObj, metav1.CreateOptions{})
	if err != nil {
		return 0, err
	}

	return createdService.Spec.Ports[0].NodePort, nil
}

func Update(clientset *kubernetes.Clientset, deployment *models.Deployment, updatedDeployment *models.Deployment) error {
	deploymentsClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)
	servicesClient := clientset.CoreV1().Services(corev1.NamespaceDefault)

	appName := fmt.Sprintf("deployment-%d-user-%d", deployment.ID, deployment.UserID)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deploymentObj, err := deploymentsClient.Get(context.TODO(), appName+"-deployment", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if updatedDeployment.Running {
			deploymentObj.Spec.Replicas = int32Ptr(updatedDeployment.Replicas)
		} else {
			deploymentObj.Spec.Replicas = int32Ptr(0)
		}

		if deployment.Volume != 0 && updatedDeployment.Volume != 0 && deployment.Volume != updatedDeployment.Volume {
			persistentVolumesClient := clientset.CoreV1().PersistentVolumes()
			persistentVolumeClaimsClient := clientset.CoreV1().PersistentVolumeClaims(corev1.NamespaceDefault)

			volume := fmt.Sprintf("%dGi", updatedDeployment.Volume)

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				pvObj, err := persistentVolumesClient.Get(context.TODO(), appName+"-pv", metav1.GetOptions{})
				if err != nil {
					return err
				}

				pvObj.Spec.Capacity = corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(volume),
				}

				_, err = persistentVolumesClient.Update(context.TODO(), pvObj, metav1.UpdateOptions{})
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				return err
			}

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				pvcObj, err := persistentVolumeClaimsClient.Get(context.TODO(), appName+"-pv-claim", metav1.GetOptions{})
				if err != nil {
					return err
				}

				pvcObj.Spec.Resources.Requests = corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(volume),
				}

				_, err = persistentVolumeClaimsClient.Update(context.TODO(), pvcObj, metav1.UpdateOptions{})
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		if len(updatedDeployment.EnvVars) != 0 {
			env_vars := []corev1.EnvVar{}

			for _, env := range deployment.EnvVars {
				parts := strings.Split(env, "=")

				if len(parts) == 2 {
					key := parts[0]
					value := parts[1]

					env_vars = append(
						env_vars,
						corev1.EnvVar{Name: key, Value: value},
					)
				}
			}
			deploymentObj.Spec.Template.Spec.Containers[0].Env = env_vars
		}

		_, err = deploymentsClient.Update(context.TODO(), deploymentObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		serviceObj, err := servicesClient.Get(context.TODO(), appName+"-service", metav1.GetOptions{})
		if err != nil {
			return err
		}

		serviceObj.Spec.Ports[0].NodePort = updatedDeployment.Port

		_, err = servicesClient.Update(context.TODO(), serviceObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func Delete(clientset *kubernetes.Clientset, id int64, userID int64) error {
	appName := fmt.Sprintf("deployment-%d-user-%d", id, userID)

	deletePolicy := metav1.DeletePropagationForeground

	err := clientset.CoreV1().PersistentVolumes().Delete(context.TODO(), appName+"-pv", metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		return err
	}

	err = clientset.CoreV1().PersistentVolumeClaims(corev1.NamespaceDefault).Delete(context.TODO(), appName+"-pv-claim", metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		return err
	}

	err = clientset.CoreV1().Services(corev1.NamespaceDefault).Delete(context.TODO(), appName+"-service", metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		return err
	}

	err = clientset.AppsV1().Deployments(corev1.NamespaceDefault).Delete(context.TODO(), appName+"-deployment", metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		return err
	}

	return nil
}

func int32Ptr(i int32) *int32 { return &i }
