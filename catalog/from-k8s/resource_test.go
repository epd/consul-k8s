package catalog

import (
	"testing"
	"time"

	"github.com/hashicorp/consul-k8s/helper/controller"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func init() {
	hclog.DefaultOptions.Level = hclog.Debug
}

func TestServiceResource_impl(t *testing.T) {
	var _ controller.Resource = &ServiceResource{}
	var _ controller.Backgrounder = &ServiceResource{}
}

// Test that deleting a service properly deletes the registration.
func TestServiceResource_createDelete(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(testService("foo"))
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Delete
	require.NoError(client.CoreV1().Services(metav1.NamespaceDefault).Delete("foo", nil))
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 0)
}

// Test that we're default enabled.
func TestServiceResource_defaultEnable(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(testService("foo"))
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
}

// Test that we can explicitly disable.
func TestServiceResource_defaultEnableDisable(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Annotations[annotationServiceSync] = "false"
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 0)
}

// Test that we can default disable
func TestServiceResource_defaultDisable(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:            hclog.Default(),
		Client:         client,
		Syncer:         syncer,
		ExplicitEnable: true,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 0)
}

// Test that we can default disable but override
func TestServiceResource_defaultDisableEnable(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:            hclog.Default(),
		Client:         client,
		Syncer:         syncer,
		ExplicitEnable: true,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Annotations[annotationServiceSync] = "t"
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
}

// Test that system resources are not synced by default.
func TestServiceResource_system(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	_, err := client.CoreV1().Services(metav1.NamespaceSystem).Create(svc)
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 0)
}

// Test that external IPs take priority.
func TestServiceResource_externalIP(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Spec: apiv1.ServiceSpec{
			Type:        apiv1.ServiceTypeLoadBalancer,
			ExternalIPs: []string{"a", "b"},
		},

		Status: apiv1.ServiceStatus{
			LoadBalancer: apiv1.LoadBalancerStatus{
				Ingress: []apiv1.LoadBalancerIngress{
					apiv1.LoadBalancerIngress{
						IP: "1.2.3.4",
					},
				},
			},
		},
	})
	require.NoError(err)

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 2)
	require.Equal("foo", actual[0].Service.Service)
	require.Equal("a", actual[0].Service.Address)
	require.Equal("foo", actual[1].Service.Service)
	require.Equal("b", actual[1].Service.Address)
}

// Test that the proper registrations are generated for a LoadBalancer.
func TestServiceResource_lb(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeLoadBalancer,
		},

		Status: apiv1.ServiceStatus{
			LoadBalancer: apiv1.LoadBalancerStatus{
				Ingress: []apiv1.LoadBalancerIngress{
					apiv1.LoadBalancerIngress{
						IP: "1.2.3.4",
					},
				},
			},
		},
	})
	require.NoError(err)

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
	require.Equal("foo", actual[0].Service.Service)
	require.Equal("1.2.3.4", actual[0].Service.Address)
}

// Test that the proper registrations are generated for a LoadBalancer
// with multiple endpoints.
func TestServiceResource_lbMultiEndpoint(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeLoadBalancer,
		},

		Status: apiv1.ServiceStatus{
			LoadBalancer: apiv1.LoadBalancerStatus{
				Ingress: []apiv1.LoadBalancerIngress{
					apiv1.LoadBalancerIngress{
						IP: "1.2.3.4",
					},
					apiv1.LoadBalancerIngress{
						IP: "2.3.4.5",
					},
				},
			},
		},
	})
	require.NoError(err)

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 2)
	require.Equal("foo", actual[0].Service.Service)
	require.Equal("1.2.3.4", actual[0].Service.Address)
	require.Equal("foo", actual[1].Service.Service)
	require.Equal("2.3.4.5", actual[1].Service.Address)
	require.NotEqual(actual[1].Service.ID, actual[0].Service.ID)
}

// Test explicit name annotation
func TestServiceResource_lbAnnotatedName(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Annotations[annotationServiceName] = "bar"
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
	require.Equal("bar", actual[0].Service.Service)
}

// Test default port and additional ports in the meta
func TestServiceResource_lbPort(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Spec.Ports = []apiv1.ServicePort{
		apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)},
		apiv1.ServicePort{Name: "rpc", Port: 8500, TargetPort: intstr.FromInt(2000)},
	}
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
	require.Equal(80, actual[0].Service.Port)
	require.Equal("80", actual[0].Service.Meta["port-http"])
	require.Equal("8500", actual[0].Service.Meta["port-rpc"])
}

// Test default port works with override annotation
func TestServiceResource_lbAnnotatedPort(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Annotations[annotationServicePort] = "rpc"
	svc.Spec.Ports = []apiv1.ServicePort{
		apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)},
		apiv1.ServicePort{Name: "rpc", Port: 8500, TargetPort: intstr.FromInt(2000)},
	}
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
	require.Equal(8500, actual[0].Service.Port)
	require.Equal("80", actual[0].Service.Meta["port-http"])
	require.Equal("8500", actual[0].Service.Meta["port-rpc"])
}

// Test annotated tags
func TestServiceResource_lbAnnotatedTags(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Annotations[annotationServiceTags] = "one, two,three"
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
	require.Equal([]string{"k8s", "one", "two", "three"}, actual[0].Service.Tags)
}

// Test annotated service meta
func TestServiceResource_lbAnnotatedMeta(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert an LB service
	svc := testService("foo")
	svc.Annotations[annotationServiceMetaPrefix+"foo"] = "bar"
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(svc)
	require.NoError(err)
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 1)
	require.Equal("bar", actual[0].Service.Meta["foo"])
}

// Test that the proper registrations are generated for a NodePort type.
func TestServiceResource_nodePort(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()

	// Insert the service
	_, err := client.CoreV1().Services(metav1.NamespaceDefault).Create(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)},
				apiv1.ServicePort{Name: "rpc", Port: 8500, TargetPort: intstr.FromInt(2000)},
			},
		},
	})
	require.NoError(err)

	// Wait a bit
	time.Sleep(300 * time.Millisecond)

	// Insert the endpoints
	_, err = client.CoreV1().Endpoints(metav1.NamespaceDefault).Create(&apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Subsets: []apiv1.EndpointSubset{
			apiv1.EndpointSubset{
				Addresses: []apiv1.EndpointAddress{
					apiv1.EndpointAddress{IP: "1.2.3.4"},
				},
			},

			apiv1.EndpointSubset{
				Addresses: []apiv1.EndpointAddress{
					apiv1.EndpointAddress{IP: "2.3.4.5"},
				},
			},
		},
	})
	require.NoError(err)

	// Wait a bit
	time.Sleep(300 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 2)
	require.Equal("foo", actual[0].Service.Service)
	require.Equal("1.2.3.4", actual[0].Service.Address)
	require.Equal("foo", actual[1].Service.Service)
	require.Equal("2.3.4.5", actual[1].Service.Address)
	require.NotEqual(actual[0].Service.ID, actual[1].Service.ID)
}

// Test that a NodePort created earlier works (doesn't require an Endpoints
// update event).
func TestServiceResource_nodePortInitial(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := fake.NewSimpleClientset()
	syncer := &TestSyncer{}

	// Start the controller
	closer := controller.TestControllerRun(&ServiceResource{
		Log:    hclog.Default(),
		Client: client,
		Syncer: syncer,
	})
	defer closer()
	time.Sleep(100 * time.Millisecond)

	// Insert the endpoints
	_, err := client.CoreV1().Endpoints(metav1.NamespaceDefault).Create(&apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Subsets: []apiv1.EndpointSubset{
			apiv1.EndpointSubset{
				Addresses: []apiv1.EndpointAddress{
					apiv1.EndpointAddress{IP: "1.2.3.4"},
				},
			},

			apiv1.EndpointSubset{
				Addresses: []apiv1.EndpointAddress{
					apiv1.EndpointAddress{IP: "2.3.4.5"},
				},
			},
		},
	})
	require.NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Insert the service
	_, err = client.CoreV1().Services(metav1.NamespaceDefault).Create(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},

		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)},
				apiv1.ServicePort{Name: "rpc", Port: 8500, TargetPort: intstr.FromInt(2000)},
			},
		},
	})
	require.NoError(err)

	// Wait a bit
	time.Sleep(400 * time.Millisecond)

	// Verify what we got
	syncer.Lock()
	defer syncer.Unlock()
	actual := syncer.Registrations
	require.Len(actual, 2)
	require.Equal("foo", actual[0].Service.Service)
	require.Equal("1.2.3.4", actual[0].Service.Address)
	require.Equal("foo", actual[1].Service.Service)
	require.Equal("2.3.4.5", actual[1].Service.Address)
}

// testService returns a service that will result in a registration.
func testService(name string) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{},
		},

		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeLoadBalancer,
		},

		Status: apiv1.ServiceStatus{
			LoadBalancer: apiv1.LoadBalancerStatus{
				Ingress: []apiv1.LoadBalancerIngress{
					apiv1.LoadBalancerIngress{
						IP: "1.2.3.4",
					},
				},
			},
		},
	}
}