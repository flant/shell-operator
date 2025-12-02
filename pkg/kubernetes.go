package pkg

import (
	"github.com/flant/shell-operator/pkg/app"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DefaultCreateOptions() metav1.CreateOptions {
	opts := metav1.CreateOptions{}

	if app.KubeClientFieldManager != "" {
		opts.FieldManager = app.KubeClientFieldManager
	}

	return opts
}

func DefaultUpdateOptions() metav1.UpdateOptions {
	opts := metav1.UpdateOptions{}

	if app.KubeClientFieldManager != "" {
		opts.FieldManager = app.KubeClientFieldManager
	}

	return opts
}

func DefaultPatchOptions() metav1.PatchOptions {
	opts := metav1.PatchOptions{}

	if app.KubeClientFieldManager != "" {
		opts.FieldManager = app.KubeClientFieldManager
	}

	return opts
}
