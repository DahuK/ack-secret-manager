package secretstore

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/AliyunContainerService/ack-secret-manager/pkg/apis/alibabacloud/v1alpha1"
	"github.com/AliyunContainerService/ack-secret-manager/pkg/backend"
	kmsprovider "github.com/AliyunContainerService/ack-secret-manager/pkg/backend/provider/kms"
	"github.com/AliyunContainerService/ack-secret-manager/pkg/utils"
)

func (r *SecretStoreReconciler) ReconcileKMS(ctx context.Context, log logr.Logger, secretStore *v1alpha1.SecretStore) (ctrl.Result, error) {
	kmsProvider := backend.GetProviderByName(backend.ProviderKMSName)
	clientName := fmt.Sprintf("%s/%s", secretStore.Namespace, secretStore.Name)
	isSecretStoretMarkedToBeDeleted := secretStore.GetDeletionTimestamp() != nil
	if isSecretStoretMarkedToBeDeleted {
		log.Info("SecretStore kms is marked to be deleted")
		if utils.Contains(secretStore.GetFinalizers(), secretFinalizer) {
			// exec the clean work in secretFinalizer
			// do not delete Finalizer if clean failed, the clean work will exec in next reconcile
			kmsProvider.Delete(clientName)
			// remove secretFinalizer
			log.Info("removing finalizer", "currentFinalizers", secretStore.GetFinalizers())
			secretStore.SetFinalizers(utils.Remove(secretStore.GetFinalizers(), secretFinalizer))
			err := r.Update(context.TODO(), secretStore)
			if err != nil {
				log.Error(err, "failed to update externalSec when clean finalizers")
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	if !utils.Contains(secretStore.GetFinalizers(), secretFinalizer) {
		if err := r.addFinalizer(log, secretStore); err != nil {
			return ctrl.Result{}, err
		}
	}

	secretClient, err := kmsProvider.NewClient(ctx, secretStore, r.Client)
	if err != nil {
		log.Error(err, fmt.Sprintf("could not new kms client %s", clientName))
		return ctrl.Result{}, err
	}
	kmsClient, ok := secretClient.(*kmsprovider.KMSClient)
	if !ok {
		klog.Errorf("client type error")
		return ctrl.Result{}, err
	}
	kmsProvider.Register(kmsClient.GetName(), kmsClient)
	return ctrl.Result{}, nil
}
