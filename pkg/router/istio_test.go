package router

import (
	"fmt"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestIstioRouter_Sync(t *testing.T) {
	mocks := setupfakeClients()
	router := &IstioRouter{
		logger:        mocks.logger,
		flaggerClient: mocks.flaggerClient,
		istioClient:   mocks.istioClient,
		kubeClient:    mocks.kubeClient,
	}

	err := router.Sync(mocks.canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	// test insert
	vs, err := mocks.istioClient.NetworkingV1alpha3().VirtualServices("default").Get("podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(vs.Spec.Http) != 1 {
		t.Errorf("Got Istio VS Http %v wanted %v", len(vs.Spec.Http), 1)
	}

	if len(vs.Spec.Http[0].Route) != 2 {
		t.Errorf("Got Istio VS routes %v wanted %v", len(vs.Spec.Http[0].Route), 2)
	}

	// test update
	cd, err := mocks.flaggerClient.FlaggerV1alpha3().Canaries("default").Get("podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err.Error())
	}

	cdClone := cd.DeepCopy()
	hosts := cdClone.Spec.Service.Hosts
	hosts = append(hosts, "test.example.com")
	cdClone.Spec.Service.Hosts = hosts
	canary, err := mocks.flaggerClient.FlaggerV1alpha3().Canaries("default").Update(cdClone)
	if err != nil {
		t.Fatal(err.Error())
	}

	// apply change
	err = router.Sync(canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	// verify
	vs, err = mocks.istioClient.NetworkingV1alpha3().VirtualServices("default").Get("podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(vs.Spec.Hosts) != 2 {
		t.Errorf("Got Istio VS hosts %v wanted %v", vs.Spec.Hosts, 2)
	}

	// test drift
	vsClone := vs.DeepCopy()
	gateways := vsClone.Spec.Gateways
	gateways = append(gateways, "test-gateway.istio-system")
	vsClone.Spec.Gateways = gateways

	vsGateways, err := mocks.istioClient.NetworkingV1alpha3().VirtualServices("default").Update(vsClone)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(vsGateways.Spec.Gateways) != 2 {
		t.Errorf("Got Istio VS gateway %v wanted %v", vsGateways.Spec.Gateways, 2)
	}

	// undo change
	err = router.Sync(mocks.canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	// verify
	vs, err = mocks.istioClient.NetworkingV1alpha3().VirtualServices("default").Get("podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(vs.Spec.Gateways) != 1 {
		t.Errorf("Got Istio VS gateways %v wanted %v", vs.Spec.Gateways, 1)
	}
}

func TestIstioRouter_SetRoutes(t *testing.T) {
	mocks := setupfakeClients()
	router := &IstioRouter{
		logger:        mocks.logger,
		flaggerClient: mocks.flaggerClient,
		istioClient:   mocks.istioClient,
		kubeClient:    mocks.kubeClient,
	}

	err := router.Sync(mocks.canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	p, c, err := router.GetRoutes(mocks.canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	p = 50
	c = 50

	err = router.SetRoutes(mocks.canary, p, c)
	if err != nil {
		t.Fatal(err.Error())
	}

	vs, err := mocks.istioClient.NetworkingV1alpha3().VirtualServices("default").Get("podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err.Error())
	}

	pRoute := istiov1alpha3.DestinationWeight{}
	cRoute := istiov1alpha3.DestinationWeight{}

	for _, http := range vs.Spec.Http {
		for _, route := range http.Route {
			if route.Destination.Host == fmt.Sprintf("%s-primary", mocks.canary.Spec.TargetRef.Name) {
				pRoute = route
			}
			if route.Destination.Host == fmt.Sprintf("%s-canary", mocks.canary.Spec.TargetRef.Name) {
				cRoute = route
			}
		}
	}

	if pRoute.Weight != p {
		t.Errorf("Got primary weight %v wanted %v", pRoute.Weight, p)
	}

	if cRoute.Weight != c {
		t.Errorf("Got canary weight %v wanted %v", cRoute.Weight, c)
	}
}

func TestIstioRouter_GetRoutes(t *testing.T) {
	mocks := setupfakeClients()
	router := &IstioRouter{
		logger:        mocks.logger,
		flaggerClient: mocks.flaggerClient,
		istioClient:   mocks.istioClient,
		kubeClient:    mocks.kubeClient,
	}

	err := router.Sync(mocks.canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	p, c, err := router.GetRoutes(mocks.canary)
	if err != nil {
		t.Fatal(err.Error())
	}

	if p != 100 {
		t.Errorf("Got primary weight %v wanted %v", p, 100)
	}

	if c != 0 {
		t.Errorf("Got canary weight %v wanted %v", c, 0)
	}
}