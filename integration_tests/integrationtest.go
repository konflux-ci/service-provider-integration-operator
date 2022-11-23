package integrationtests

import (
	"context"
	"reflect"
	"time"

	"github.com/onsi/ginkgo"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/util/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hashicorp/vault/vault"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// ITest is the globally accessible integration test "context"
var ITest = IntegrationTest{}

// IntegrationTest is meant to be used through the ITest global variable to inspect and configure the behavior of the
// various subsystems of SPI.
type IntegrationTest struct {
	// Client is the Kubernetes client to use to talk to the Kubernetes cluster
	Client client.Client
	// NoPrivsClient is a Kubernetes client to use to talk to the Kubernetes cluster that doesn't have any permissions
	NoPrivsClient client.Client
	// TestEnvironment is the Kubernetes API abstraction that we're using to simulate a full-blown cluster
	TestEnvironment *envtest.Environment
	// Context is the context to use with various API requests. It is set up with timeout cancelling to correctly handle
	// the testsuite timeouts. Use Cancel to force the cancellation of the context yourself, if ever needed.
	Context context.Context
	// TokenStorage is the token storage instance that the controllers are using to store the token data. By default,
	// this is backed the VaultTestCluster.
	TokenStorage tokenstorage.TokenStorage
	// Cancel can be used to forcefully cancel the Context, interrupting all the future requests and thus short-circuiting
	// the testsuite as a whole.
	Cancel context.CancelFunc
	// TestServiceProviderProbe is the probing function to identify the service provider to use. This is automagically
	// setup to recognize the URLs starting with "test-provider://" as handled by the TestServiceProvider.
	TestServiceProviderProbe serviceprovider.Probe
	// TestServiceProvider is the service provider that the controllers are set up to use. You can modify its behavior
	// in the before-each of the tests.
	TestServiceProvider TestServiceProvider
	// HostCredsServiceProvider is the fallback provider used when no other service provider is detected for given URL.
	HostCredsServiceProvider TestServiceProvider
	// VaultTestCluster is Vault's in-memory test cluster instance.
	VaultTestCluster *vault.TestCluster
	// OperatorConfiguration is the "live" configuration used by the controllers. Changing the values here has direct
	// effect in the controllers as long as they don't cache the values somehow (by storing them in an instance field
	// for example).
	OperatorConfiguration *opconfig.OperatorConfiguration
	// MetricsRegistry is the metrics registry the controllers are configured with. This can be used to check that the
	// metrics are being collected.
	MetricsRegistry *prometheus.Registry
}

// TestSetup is used to express the requirements on the state of the K8s Cluster before the tests. Once an instance with
// the desired configuration is produced, its BeforeEach and AfterEach methods can be called to bring the cluster to
// the desired state and tear it back down.
type TestSetup struct {
	// ToCreate is a list of objects that are expected to be present in the cluster. Once BeforeEach is called, the
	// true state of those objects is stored in the InCluster field.
	ToCreate TestObjects
	// InCluster references all the objects (that we're interested in) that exist in the cluster. It is filled in during
	// the BeforeEach method and represents the true state of the objects (no need to load them again after BeforeEach
	// completes).
	InCluster TestObjects
	// Behavior is used to set up the behavior of the ITest at various stages (you can modify the service providers,
	// configuration, etc.)
	Behavior ITestBehavior
	// Timing configures the different periods and TTLs desired. By default, everything is set up to never expire so
	// that the test methods don't need to take into account the disappearance of objects due to unpredictable timing
	// issues.
	Timing ITestTiming

	priorState priorITestState
}

// ITestTiming collects all the timing configuration. The changes made in ITestBehavior methods (if any) take precedence
// over what is configured here.
type ITestTiming struct {
	// Tokens is the TTL of the tokens
	Tokens time.Duration
	// Bindings is the TTL of the bindings
	Bindings time.Duration
	// Checks is the TTL of the SPIAccessChecks
	Checks time.Duration
	// FileRequests is the TTL of the SPIFileContentRequests
	FileRequests time.Duration
	// TokenLookupCache is the TTL of the token metadata
	TokenLookupCache time.Duration
	// DeletionGracePeriod is the grace period before tokens in awaiting state are deleted
	DeletionGracePeriod time.Duration
}

// ITestBehavior configures the ITest for the tests.
type ITestBehavior struct {
	// BeforeObjectsCreated sets up the behavior before any of the desired objects specified in TestSetup.ToCreate are
	// actually created.
	BeforeObjectsCreated func()
	// AfterObjectsCreated sets up the behavior after the objects from TestSetup.ToCreate (and possibly others, like
	// auto-created tokens for the bindings) have been created. The objects currently existing in the cluster are passed
	// in as an argument.
	AfterObjectsCreated func(TestObjects)
	// DontTriggerReconcileAfterObjectsCreated in the unlikely event, where you DON'T want to trigger the reconciliation
	// of the objects in the cluster after the ITest behavior was changed in AfterObjectsCreated, set this to true.
	DontTriggerReconcileAfterObjectsCreated bool
}

// TestObjects collects the objects of interest as they are required or exist in the cluster
type TestObjects struct {
	Tokens              []*api.SPIAccessToken
	Bindings            []*api.SPIAccessTokenBinding
	Checks              []*api.SPIAccessCheck
	FileContentRequests []*api.SPIFileContentRequest
	DataUpdates         []*api.SPIAccessTokenDataUpdate
}

// priorITestState is used internally to store the state of the ITest as it existed before the test. The ITest is
// restored to this state in TestSetup.AfterEach. These are essentially just copies of the ITest objects.
type priorITestState struct {
	probe             serviceprovider.Probe
	serviceProvider   TestServiceProvider
	hostCredsProvider TestServiceProvider
	operatorConfig    opconfig.OperatorConfiguration // intentionally not a pointer

	// we're missing the configuration of token storage here, because there's no way I know of to reconstruct it after
	// ITestBehavior modifies it. We'd need to go the way of the TestServiceProvider so that we have a struct that we
	// can copy.
}

// StandardTestToken creates an SPIAccessToken with the configuration commonly used in the tests.
func StandardTestToken(namePrefix string) *api.SPIAccessToken {
	return &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix + "-",
			Namespace:    "default",
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: "test-provider://",
		},
	}
}

// StandardTestBinding creates an SPIAccessTokenBinding with the configuration commonly used in the tests.
func StandardTestBinding(namePrefix string) *api.SPIAccessTokenBinding {
	return &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix + "-binding-",
			Namespace:    "default",
		},
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "test-provider://test",
		},
	}
}

// TriggerReconciliation updates the provided object with a "random-annon-to-trigger-reconcile" annotation (with
// a random value) so that a new reconciliation is performed.
func TriggerReconciliation(object client.Object) {
	Eventually(func(g Gomega) {
		// trigger the update of the token to force the reconciliation
		cpy := object.DeepCopyObject().(client.Object)
		g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(object), cpy)).To(Succeed())
		annos := object.GetAnnotations()
		if annos == nil {
			annos = map[string]string{}
		}
		annos["random-anno-to-trigger-reconcile"] = string(uuid.NewUUID())
		cpy.SetAnnotations(annos)
		g.Expect(ITest.Client.Update(ITest.Context, cpy)).To(Succeed())
	}).Should(Succeed())
}

// BeforeEach is where the magic happens. It first checks that the cluster is empty, then stores the configuration
// of the ITest, resets it, creates the required objects, re-configures the ITest and waits for the cluster state to
// settle (i.e. wait for the controllers to create all the additional objects and finish all the reconciles). Once this
// method returns, the TestSetup.InCluster contains the objects of interest as they exist in the cluster after all
// the reconciliation has been performed at least once with the reconfigured ITest.
//
// NOTE we're not doing anything with the metrics registry so far here...
func (ts *TestSetup) BeforeEach() {
	validateClusterEmpty()

	ts.priorState = priorITestState{
		probe:             ITest.TestServiceProviderProbe,
		serviceProvider:   ITest.TestServiceProvider,
		hostCredsProvider: ITest.HostCredsServiceProvider,
		operatorConfig:    *ITest.OperatorConfiguration,
	}

	ITest.TestServiceProvider.Reset()
	ITest.OperatorConfiguration.AccessTokenTtl = infiniteIfZero(ts.Timing.Tokens)
	ITest.OperatorConfiguration.AccessTokenBindingTtl = infiniteIfZero(ts.Timing.Bindings)
	ITest.OperatorConfiguration.AccessCheckTtl = infiniteIfZero(ts.Timing.Checks)
	ITest.OperatorConfiguration.FileContentRequestTtl = infiniteIfZero(ts.Timing.FileRequests)
	ITest.OperatorConfiguration.TokenLookupCacheTtl = infiniteIfZero(ts.Timing.TokenLookupCache)
	ITest.OperatorConfiguration.DeletionGracePeriod = infiniteIfZero(ts.Timing.DeletionGracePeriod)

	if ts.Behavior.BeforeObjectsCreated != nil {
		ts.Behavior.BeforeObjectsCreated()
	}

	ts.InCluster = TestObjects{
		Tokens:              createAll(ts.ToCreate.Tokens),
		Bindings:            createAll(ts.ToCreate.Bindings),
		Checks:              createAll(ts.ToCreate.Checks),
		FileContentRequests: createAll(ts.ToCreate.FileContentRequests),
		DataUpdates:         createAll(ts.ToCreate.DataUpdates),
	}

	// we don't need to force reconcile here, because we just created the objects so a reconciliation is running..
	ts.settleWithCluster(false)

	if ts.Behavior.AfterObjectsCreated != nil {
		ts.Behavior.AfterObjectsCreated(ts.InCluster)

		if !ts.Behavior.DontTriggerReconcileAfterObjectsCreated {
			ts.settleWithCluster(true)
		}
	}

	log.Log.Info("============================================================================")
	log.Log.Info("================ All objects created and cluster state settled =============")
	log.Log.Info("=== For test: " + ginkgo.CurrentGinkgoTestDescription().FullTestText)
	log.Log.Info("============================================================================")
}

// AfterEach cleans up all the objects from the cluster and reverts the behavior of ITest to what it was before the test
// started (to what BeforeEach stored).
func (ts *TestSetup) AfterEach() {
	// clean up all in the reverse direction to BeforeEach
	deleteAll(ts.InCluster.DataUpdates)
	deleteAll(ts.InCluster.FileContentRequests)
	deleteAll(ts.InCluster.Checks)
	deleteAll(ts.InCluster.Bindings)
	deleteAll(ts.InCluster.Tokens)

	ts.InCluster = TestObjects{}

	ITest.TestServiceProviderProbe = ts.priorState.probe
	ITest.TestServiceProvider = ts.priorState.serviceProvider
	ITest.HostCredsServiceProvider = ts.priorState.hostCredsProvider
	// we must keep the address of the operator configuration because that's the pointer the controllers are set up with
	// we're just changing the contents of the objects that the pointer is pointing to...
	*ITest.OperatorConfiguration = ts.priorState.operatorConfig

	validateClusterEmpty()
}

// SettleWithCluster triggers the reconciliation and waits for the cluster to settle again. This can be used after
// a test or a nested Gomega.BeforeEach modifies the behavior and we need to re-sync and wait for the controllers to
// accommodate for the changed behavior.
func (ts *TestSetup) SettleWithCluster() {
	ts.settleWithCluster(true)
}

func (ts *TestSetup) settleWithCluster(forceReconcile bool) {
	if forceReconcile {
		forEach(ts.InCluster.Tokens, TriggerReconciliation)
		forEach(ts.InCluster.Bindings, TriggerReconciliation)
		forEach(ts.InCluster.Checks, TriggerReconciliation)
		forEach(ts.InCluster.DataUpdates, TriggerReconciliation)
		forEach(ts.InCluster.FileContentRequests, TriggerReconciliation)
	}

	waitForStatus := func() {
		waitForStatus(&api.SPIAccessTokenList{}, convertTokens, emptyTokenStatus)
		waitForStatus(&api.SPIAccessTokenBindingList{}, convertBindings, emptyBindingStatus)
		waitForStatus(&api.SPIAccessCheckList{}, convertChecks, emptyCheckStatus)
		waitForStatus(&api.SPIFileContentRequestList{}, convertFiles, emptyFileStatus)
		waitForStatus(&api.SPIAccessTokenDataUpdateList{}, convertDataUpdates, emptyDataUpdateStatus)
	}

	findAll := func() {
		ts.InCluster.Tokens = addAllExisting(&api.SPIAccessTokenList{}, convertTokens)
		ts.InCluster.Bindings = addAllExisting(&api.SPIAccessTokenBindingList{}, convertBindings)
		ts.InCluster.Checks = addAllExisting(&api.SPIAccessCheckList{}, convertChecks)
		ts.InCluster.FileContentRequests = addAllExisting(&api.SPIFileContentRequestList{}, convertFiles)
		ts.InCluster.DataUpdates = addAllExisting(&api.SPIAccessTokenDataUpdateList{}, convertDataUpdates)
	}

	// we need to create copies of the objects from the cluster, so these are arrays of structs, not arrays of pointers
	// to structs.
	var tokens []api.SPIAccessToken
	var bindings []api.SPIAccessTokenBinding
	var checks []api.SPIAccessCheck
	var fileRequests []api.SPIFileContentRequest
	var dataUpdates []api.SPIAccessTokenDataUpdate

	loadFromCluster := func() {
		tokens = fromPointerArray(ts.InCluster.Tokens)
		bindings = fromPointerArray(ts.InCluster.Bindings)
		checks = fromPointerArray(ts.InCluster.Checks)
		fileRequests = fromPointerArray(ts.InCluster.FileContentRequests)
		dataUpdates = fromPointerArray(ts.InCluster.DataUpdates)
	}

	for {
		loadFromCluster()
		waitForStatus()
		findAll()

		if hasAnyChanged(toPointerArray(tokens), ts.InCluster.Tokens) ||
			hasAnyChanged(toPointerArray(bindings), ts.InCluster.Bindings) ||
			hasAnyChanged(toPointerArray(checks), ts.InCluster.Checks) ||
			hasAnyChanged(toPointerArray(fileRequests), ts.InCluster.FileContentRequests) ||
			hasAnyChanged(toPointerArray(dataUpdates), ts.InCluster.DataUpdates) {

			time.Sleep(200 * time.Millisecond)
			continue
		}

		break
	}
}

func infiniteIfZero(dur time.Duration) time.Duration {
	if dur == 0 {
		// a year is infinite enough
		return 365 * 24 * time.Hour
	}

	return dur
}

func forEach[T client.Object](list []T, fn func(client.Object)) {
	for _, o := range list {
		fn(o)
	}
}

func convertTokens(l *api.SPIAccessTokenList) []*api.SPIAccessToken {
	return toPointerArray(l.Items)
}

func emptyTokenStatus(t *api.SPIAccessToken) bool {
	return t.Status.Phase == ""
}

func convertBindings(l *api.SPIAccessTokenBindingList) []*api.SPIAccessTokenBinding {
	return toPointerArray(l.Items)
}

func emptyBindingStatus(t *api.SPIAccessTokenBinding) bool {
	return t.Status.Phase == ""
}

func convertChecks(l *api.SPIAccessCheckList) []*api.SPIAccessCheck {
	return toPointerArray(l.Items)
}

func emptyCheckStatus(t *api.SPIAccessCheck) bool {
	return t.Status.ServiceProvider == ""
}

func convertFiles(l *api.SPIFileContentRequestList) []*api.SPIFileContentRequest {
	return toPointerArray(l.Items)
}

func emptyFileStatus(t *api.SPIFileContentRequest) bool {
	return t.Status.Phase == ""
}

func convertDataUpdates(l *api.SPIAccessTokenDataUpdateList) []*api.SPIAccessTokenDataUpdate {
	return toPointerArray(l.Items)
}

func emptyDataUpdateStatus(t *api.SPIAccessTokenDataUpdate) bool {
	return false
}

func validateClusterEmpty() {
	// TODO the rest of the testsuite leaves garbage in the cluster, so let's switch this off until everything is converted
	//checkNoObjectsInCluster(&api.SPIAccessTokenList{}, func(l *api.SPIAccessTokenList) int {
	//	return len(l.Items)
	//})
	//checkNoObjectsInCluster(&api.SPIAccessTokenBindingList{}, func(l *api.SPIAccessTokenBindingList) int {
	//	return len(l.Items)
	//})
	//checkNoObjectsInCluster(&api.SPIAccessCheckList{}, func(l *api.SPIAccessCheckList) int {
	//	return len(l.Items)
	//})
	//checkNoObjectsInCluster(&api.SPIAccessTokenDataUpdateList{}, func(l *api.SPIAccessTokenDataUpdateList) int {
	//	return len(l.Items)
	//})
	//checkNoObjectsInCluster(&api.SPIFileContentRequestList{}, func(l *api.SPIFileContentRequestList) int {
	//	return len(l.Items)
	//})
}

func checkNoObjectsInCluster[L client.ObjectList](list L, getLen func(l L) int) {
	Eventually(func(g Gomega) {
		g.Expect(ITest.Client.List(ITest.Context, list)).To(Succeed())
		g.Expect(getLen(list)).To(BeZero())
	}).Should(Succeed())
}

func createAll[T client.Object](objs []T) []T {
	ret := make([]T, 0, len(objs))
	for _, proto := range objs {
		o := proto.DeepCopyObject().(T)
		Expect(ITest.Client.Create(ITest.Context, o)).To(Succeed())
		ret = append(ret, o)
	}

	return ret
}

func deleteAll[T client.Object](objs []T) {
	for _, o := range objs {
		Eventually(func(g Gomega) {
			cpy := o.DeepCopyObject().(client.Object)
			err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(o), cpy)
			// tolerate if the object is no longer found because it was for example deleted during the test
			if err != nil {
				if !errors.IsNotFound(err) {
					g.Expect(err).To(Succeed())
				}
				return
			}

			err = ITest.Client.Delete(ITest.Context, cpy)
			// tolerate if the object is no longer found because it was for example deleted during the test
			if err != nil && !errors.IsNotFound(err) {
				g.Expect(err).To(Succeed())
			}
		}).Should(Succeed())
	}
}

func waitForStatus[T client.Object, L client.ObjectList](list L, itemsFromList func(L) []T, hasEmptyStatus func(T) bool) {
	// inefficient but short :)
Outer:
	for {
		Expect(ITest.Client.List(ITest.Context, list)).To(Succeed())
		items := itemsFromList(list)
		if len(items) == 0 {
			return
		}
		for _, o := range items {
			if hasEmptyStatus(o) {
				time.Sleep(time.Second)
				continue Outer
			}
		}
		break
	}
}

func addAllExisting[T client.Object, L client.ObjectList](list L, itemsFromList func(L) []T) []T {
	var existing []T
	Expect(ITest.Client.List(ITest.Context, list)).To(Succeed())

	for _, o := range itemsFromList(list) {
		existing = append(existing, o)
	}

	return existing
}

func hasAnyChanged[T client.Object](origs []T, news []T) bool {
	if len(origs) != len(news) {
		return true
	}

	for i, o := range origs {
		n := news[i]
		if hasChanged(o, n) {
			return true
		}
	}

	return false
}

func hasChanged[T client.Object](a T, b T) bool {
	copyA := a.DeepCopyObject().(T)
	copyB := b.DeepCopyObject().(T)

	// remove the metadata from the objects
	copyA.SetAnnotations(nil)
	copyA.SetResourceVersion("")
	copyA.SetManagedFields(nil)
	copyB.SetAnnotations(nil)
	copyB.SetResourceVersion("")
	copyB.SetManagedFields(nil)

	return !reflect.DeepEqual(copyA, copyB)
}

func toPointerArray[T any](arr []T) []*T {
	ret := make([]*T, 0, len(arr))
	for _, o := range arr {
		ret = append(ret, &o)
	}

	return ret
}

func fromPointerArray[T any](arr []*T) []T {
	ret := make([]T, 0, len(arr))
	for _, o := range arr {
		ret = append(ret, *o)
	}

	return ret
}
