package controllers

import (
	"path/filepath"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
)

var testk8sClient client.Client
var testEnv *envtest.Environment

func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.WriteTo(nil)))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}

	if err := werfv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	testk8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}

	code := m.Run()

	if err := testEnv.Stop(); err != nil {
		panic(err)
	}

	exit(code)
}

// exit is stubbed for testing
var exit = func(code int) {}
