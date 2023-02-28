//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrationtests

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	"github.com/go-test/deep"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

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
	TestServiceProvider serviceprovider.TestServiceProvider
	// Capabilities is a pluggable implementation of the capabilities that can implemented by the service providers.
	// Note that by default the TestServiceProvider is NOT set up to return this instance (i.e. by default, the test
	// service provider doesn't support any additional capabilities).
	// This instance is set up with the default implementations of the methods so that the callers don't have to set
	// them up if they don't need to.
	Capabilities serviceprovider.TestCapabilities
	// HostCredsServiceProvider is the fallback provider used when no other service provider is detected for given URL.
	HostCredsServiceProvider serviceprovider.TestServiceProvider
	// VaultTestCluster is Vault's in-memory test cluster instance.
	VaultTestCluster *vault.TestCluster
	// OperatorConfiguration is the "live" configuration used by the controllers. Changing the values here has direct
	// effect in the controllers as long as they don't cache the values somehow (by storing them in an instance field
	// for example).
	OperatorConfiguration *opconfig.OperatorConfiguration
	// MetricsRegistry is the metrics registry the controllers are configured with. This can be used to check that the
	// metrics are being collected.
	MetricsRegistry *prometheus.Registry
	// Custom validation options to register
	ValidationOptions config.CustomValidationOptions
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

func (to TestObjects) GetToken(key client.ObjectKey) *api.SPIAccessToken {
	return findByKey(key, to.Tokens)
}

func (to TestObjects) GetTokensByNamePrefix(key client.ObjectKey) []*api.SPIAccessToken {
	return matchByNamePrefix(key, to.Tokens)
}

func (to TestObjects) GetBinding(key client.ObjectKey) *api.SPIAccessTokenBinding {
	return findByKey(key, to.Bindings)
}

func (to TestObjects) GetBindingsByNamePrefix(key client.ObjectKey) []*api.SPIAccessTokenBinding {
	return matchByNamePrefix(key, to.Bindings)
}

func (to TestObjects) GetCheck(key client.ObjectKey) *api.SPIAccessCheck {
	return findByKey(key, to.Checks)
}

func (to TestObjects) GetChecksByNamePrefix(key client.ObjectKey) []*api.SPIAccessCheck {
	return matchByNamePrefix(key, to.Checks)
}

func (to TestObjects) GetFileContentRequest(key client.ObjectKey) *api.SPIFileContentRequest {
	return findByKey(key, to.FileContentRequests)
}

func (to TestObjects) GetFileContentRequestsByNamePrefix(key client.ObjectKey) []*api.SPIFileContentRequest {
	return matchByNamePrefix(key, to.FileContentRequests)
}

func (to TestObjects) GetDataUpdate(key client.ObjectKey) *api.SPIAccessTokenDataUpdate {
	return findByKey(key, to.DataUpdates)
}

func (to TestObjects) GetDataUpdatesByNamePrefix(key client.ObjectKey) []*api.SPIAccessTokenDataUpdate {
	return matchByNamePrefix(key, to.DataUpdates)
}

func findByKey[T client.Object](key client.ObjectKey, list []T) T {
	for _, o := range list {
		if o.GetName() == key.Name && o.GetNamespace() == key.Namespace {
			return o
		}
	}

	// this is nil, because we only ever use this with pointers because only those implement client.Object
	var zero T
	return zero
}

func matchByNamePrefix[T client.Object](key client.ObjectKey, list []T) []T {
	var ret []T
	for i, o := range list {
		if strings.HasPrefix(o.GetName(), key.Name) && o.GetNamespace() == key.Namespace {
			ret = append(ret, list[i])
		}
	}

	return ret
}

// priorITestState is used internally to store the state of the ITest as it existed before the test. The ITest is
// restored to this state in TestSetup.AfterEach. These are essentially just copies of the ITest objects.
type priorITestState struct {
	probe             serviceprovider.Probe
	serviceProvider   serviceprovider.TestServiceProvider
	hostCredsProvider serviceprovider.TestServiceProvider
	operatorConfig    opconfig.OperatorConfiguration // intentionally not a pointer
	validationOptions config.CustomValidationOptions

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

func StandardFileRequest(namePrefix string) *api.SPIFileContentRequest {
	return &api.SPIFileContentRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix + "-filerequest-",
			Namespace:    "default",
		},
		Spec: api.SPIFileContentRequestSpec{
			RepoUrl:  "test-provider://test",
			FilePath: "foo/bar.txt",
		},
	}
}

// TriggerReconciliation updates the provided object with a "random-annon-to-trigger-reconcile" annotation (with
// a random value) so that a new reconciliation is performed.
func TriggerReconciliation(object client.Object) {
	Eventually(func(g Gomega) {
		// trigger the update of the token to force the reconciliation
		cpy := object.DeepCopyObject().(client.Object)
		err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(object), cpy)
		if errors.IsNotFound(err) {
			// oh, well, it's gone, no way of reconciling it...
			log.Log.V(logs.DebugLevel).Info("wanted to force reconciliation but object is gone",
				"object", client.ObjectKeyFromObject(object),
				"kind", object.GetObjectKind().GroupVersionKind().String())
			return
		}
		log.Log.V(logs.DebugLevel).Info("forcing reconciliation",
			"object", client.ObjectKeyFromObject(object),
			"kind", object.GetObjectKind().GroupVersionKind().String())

		Expect(err).NotTo(HaveOccurred())

		annos := object.GetAnnotations()
		if annos == nil {
			annos = map[string]string{}
		}
		annos["random-anno-to-trigger-reconcile"] = string(uuid.NewUUID())
		cpy.SetAnnotations(annos)
		g.Expect(ITest.Client.Update(ITest.Context, cpy)).To(Succeed())
	}).Should(Succeed())

	log.Log.V(logs.DebugLevel).Info("update to force reconciliation succeeded",
		"object", client.ObjectKeyFromObject(object),
		"kind", object.GetObjectKind().GroupVersionKind().String())

}

// BeforeEach is where the magic happens. It first checks that the cluster is empty, then stores the configuration
// of the ITest, resets it, creates the required objects, re-configures the ITest and waits for the cluster state to
// settle (i.e. wait for the controllers to create all the additional objects and finish all the reconciles). Once this
// method returns, the TestSetup.InCluster contains the objects of interest as they exist in the cluster after all
// the reconciliation has been performed at least once with the reconfigured ITest.
//
// The `postCondition` is a (potentially `nil`) check that needs to succeed before we can claim the cluster reached the
// desired state. If it is `nil`, then only the best effort is made to wait for the controllers to finish
// the reconciliation (basically the only thing guaranteed is that the objects will have a status, i.e.
// the reconciliation happened at least once).
//
// NOTE we're not doing anything with the metrics registry so far here...
func (ts *TestSetup) BeforeEach(postCondition func(Gomega)) {
	start := time.Now()

	// I've seen some timing issues where beforeeach seems to be executed in parallel with aftereach of the test before
	// which would cause problems because we assume that the tests run sequentially. Let's just wait here a little, to
	// try and clear that condition up.
	Eventually(func(g Gomega) {
		validateClusterEmpty(g)
	}).Should(Succeed())

	ts.priorState = priorITestState{
		probe:             ITest.TestServiceProviderProbe,
		serviceProvider:   ITest.TestServiceProvider,
		hostCredsProvider: ITest.HostCredsServiceProvider,
		operatorConfig:    *ITest.OperatorConfiguration,
		validationOptions: ITest.ValidationOptions,
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

	err := config.SetupCustomValidations(ITest.ValidationOptions)
	Expect(err).ShouldNot(HaveOccurred())

	ts.InCluster = TestObjects{
		Tokens:              createAll(ts.ToCreate.Tokens),
		Bindings:            createAll(ts.ToCreate.Bindings),
		Checks:              createAll(ts.ToCreate.Checks),
		FileContentRequests: createAll(ts.ToCreate.FileContentRequests),
		DataUpdates:         createAll(ts.ToCreate.DataUpdates),
	}

	// we don't need to force reconcile here, because we just created the objects so a reconciliation is running..

	if ts.Behavior.AfterObjectsCreated != nil {
		if ts.Behavior.DontTriggerReconcileAfterObjectsCreated {
			ts.settleWithCluster(false, postCondition)
			ts.Behavior.AfterObjectsCreated(ts.InCluster)
		} else {
			ts.settleWithCluster(false, nil)
			ts.Behavior.AfterObjectsCreated(ts.InCluster)
			ts.settleWithCluster(true, postCondition)
		}
	} else {
		ts.settleWithCluster(false, postCondition)
	}

	log.Log.Info("=====")
	log.Log.Info("=====")
	log.Log.Info("=====")
	log.Log.Info("=====")
	log.Log.Info("===== All objects created and cluster state settled")
	log.Log.Info("===== For test: " + ginkgo.CurrentGinkgoTestDescription().FullTestText)
	log.Log.Info(fmt.Sprintf("===== Setup complete in %dms", time.Since(start).Milliseconds()))
	log.Log.Info("=====")
	log.Log.Info("=====")
	log.Log.Info("=====")
	log.Log.Info("=====")
}

// AfterEach cleans up all the objects from the cluster and reverts the behavior of ITest to what it was before the test
// started (to what BeforeEach stored).
func (ts *TestSetup) AfterEach() {
	// clean up all in the reverse direction to BeforeEach
	deleteAll(newListDataUpdates, listLenDataUpdates, convertDataUpdates)
	deleteAll(newListFiles, listLenFiles, convertFiles)
	deleteAll(newListChecks, listLenChecks, convertChecks)
	deleteAll(newListBindings, listLenBindings, convertBindings)
	deleteAll(newListTokens, listLenTokens, convertTokens)

	ts.InCluster = TestObjects{}

	ITest.TestServiceProviderProbe = ts.priorState.probe
	ITest.TestServiceProvider = ts.priorState.serviceProvider
	ITest.HostCredsServiceProvider = ts.priorState.hostCredsProvider
	ITest.ValidationOptions = ts.priorState.validationOptions
	// we must keep the address of the operator configuration because that's the pointer the controllers are set up with
	// we're just changing the contents of the objects that the pointer is pointing to...
	*ITest.OperatorConfiguration = ts.priorState.operatorConfig

	Eventually(func(g Gomega) {
		validateClusterEmpty(g)
	}).Should(Succeed())
}

// ReconcileWithCluster triggers the reconciliation and waits for the cluster to settle again. This can be used after
// a test or a nested Gomega.BeforeEach modifies the behavior and we need to re-sync and wait for the controllers to
// accommodate for the changed behavior.
//
// The `postCondition` is a (potentially `nil`) check that needs to succeed before we can claim the cluster reached the
// desired state. If it is `nil`, then only the best effort is made to wait for the controllers to finish
// the reconciliation (basically the only thing guaranteed is that the objects will have a status, i.e.
// the reconciliation happened at least once).
//
// The `postCondition` can use the `testSetup.InCluster` to access the current state of the objects (which is being
// updated during this call).
func (ts *TestSetup) ReconcileWithCluster(postCondition func(Gomega)) {
	_, filename, line, _ := runtime.Caller(1)
	log.Log.Info("////")
	log.Log.Info("////")
	log.Log.Info("////")
	log.Log.Info("////")
	log.Log.Info("//// Triggering reconciliation with cluster")
	log.Log.Info("////", "file", filename, "line", line)
	log.Log.Info("////")
	log.Log.Info("////")
	log.Log.Info("////")
	log.Log.Info("////")

	ts.settleWithCluster(true, postCondition)

	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\ Finished reconciliation with cluster")
	log.Log.Info("\\\\\\\\", "file", filename, "line", line)
	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\")
	log.Log.Info("\\\\\\\\")
}

func (ts *TestSetup) settleWithCluster(forceReconcile bool, postCondition func(Gomega)) {
	waitForStatus := func() {
		waitForStatus(&api.SPIAccessTokenList{}, convertTokens, emptyTokenStatus, "phase")
		waitForStatus(&api.SPIAccessTokenBindingList{}, convertBindings, emptyBindingStatus, "phase")
		waitForStatus(&api.SPIAccessCheckList{}, convertChecks, emptyCheckStatus, "serviceProvider")
		waitForStatus(&api.SPIFileContentRequestList{}, convertFiles, emptyFileStatus, "phase")
		waitForStatus(&api.SPIAccessTokenDataUpdateList{}, convertDataUpdates, emptyDataUpdateStatus, "")
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

	rememberCurrentClusterState := func() {
		tokens = fromPointerArray(ts.InCluster.Tokens)
		bindings = fromPointerArray(ts.InCluster.Bindings)
		checks = fromPointerArray(ts.InCluster.Checks)
		fileRequests = fromPointerArray(ts.InCluster.FileContentRequests)
		dataUpdates = fromPointerArray(ts.InCluster.DataUpdates)
	}

	i := 0
	var lastReconcile *time.Time
	Eventually(func(g Gomega) {
		i += 1

		rememberCurrentClusterState()

		// this loop is usually very fast, so we trigger the reconciliation the first time and then only every 2s to
		// give the controllers some time to react.
		if forceReconcile && (lastReconcile == nil || time.Since(*lastReconcile) > 2*time.Second) {
			if lastReconcile != nil {
				log.Log.Info("////")
				log.Log.Info("////")
				log.Log.Info("////")
				log.Log.Info("////")
				log.Log.Info("//// Reconciliation still in progress after (another) 2s. Triggering it again.")
				log.Log.Info("////")
				log.Log.Info("////")
				log.Log.Info("////")
				log.Log.Info("////")
			}
			forEach(ts.InCluster.Tokens, TriggerReconciliation)
			forEach(ts.InCluster.Bindings, TriggerReconciliation)
			forEach(ts.InCluster.Checks, TriggerReconciliation)
			forEach(ts.InCluster.DataUpdates, TriggerReconciliation)
			forEach(ts.InCluster.FileContentRequests, TriggerReconciliation)
			now := time.Now()
			lastReconcile = &now
		}

		waitForStatus()
		findAll()

		// ok, so now we're in one of 2 possible states wrt reconciliation:
		// 1) the reconciliation happened for the first time (the objects didn't have status, and we waited for
		//    the controllers to fill it in,
		// 2) the objects have already been reconciled before and we either forced reconciliation or not. We don't know
		//    if any changes are yet to happen in the cluster as controllers react or if the reconciliation already
		//    finished.
		//
		// Therefore, we can do just 2 things: we can monitor if any change has already happened in the cluster and
		// check that the post condition is passing. Neither of those things actually guarantees that the reconciliation
		// will have happened by the time we return from this method. But that's OK. If the caller wanted to trigger
		// reconciliation, they most probably also have provided a post condition to check the desired changes have
		// actually happened. If the caller didn't provide a post condition, they have been warned - we only offer
		// waiting for reconciliation on the best effort basis.

		tokenDiffs := findDifferences(toPointerArray(tokens), ts.InCluster.Tokens)
		bindingDiffs := findDifferences(toPointerArray(bindings), ts.InCluster.Bindings)
		checkDiffs := findDifferences(toPointerArray(checks), ts.InCluster.Checks)
		fileRequestDiffs := findDifferences(toPointerArray(fileRequests), ts.InCluster.FileContentRequests)
		dataUpdateDiffs := findDifferences(toPointerArray(dataUpdates), ts.InCluster.DataUpdates)

		if len(tokenDiffs) > 0 || len(bindingDiffs) > 0 || len(checkDiffs) > 0 || len(fileRequestDiffs) > 0 || len(dataUpdateDiffs) > 0 {
			if i > 1 {
				log.Log.Info("~~~~")
				log.Log.Info("~~~~")
				log.Log.Info("~~~~")
				log.Log.Info("~~~~")
				log.Log.Info("settling loop still seeing changes",
					"iteration", i,
					"test", ginkgo.CurrentGinkgoTestDescription().FullTestText,
					"tokenDiffs", tokenDiffs,
					"bindingDiffs", bindingDiffs,
					"checkDiffs", checkDiffs,
					"fileRequestDiffs", fileRequestDiffs,
					"dataUpdateDiffs", dataUpdateDiffs,
				)
				log.Log.Info("~~~~")
				log.Log.Info("~~~~")
				log.Log.Info("~~~~")
				log.Log.Info("~~~~")
			}

			time.Sleep(200 * time.Millisecond)

			// true here is the result of the if statement that we're nested in... We expect that to be false :)
			g.Expect(true).To(BeFalse())
			// the cluster state is still evolving, no need to bother with calling postCondition yet
			return
		}

		if postCondition != nil {
			postCondition(g)
		}

		i += 1
	}).Should(Succeed())
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

func newListTokens() *api.SPIAccessTokenList {
	return &api.SPIAccessTokenList{}
}

func listLenTokens(l *api.SPIAccessTokenList) int {
	return len(l.Items)
}

func convertBindings(l *api.SPIAccessTokenBindingList) []*api.SPIAccessTokenBinding {
	return toPointerArray(l.Items)
}

func emptyBindingStatus(t *api.SPIAccessTokenBinding) bool {
	return t.Status.Phase == ""
}

func newListBindings() *api.SPIAccessTokenBindingList {
	return &api.SPIAccessTokenBindingList{}
}

func listLenBindings(l *api.SPIAccessTokenBindingList) int {
	return len(l.Items)
}

func convertChecks(l *api.SPIAccessCheckList) []*api.SPIAccessCheck {
	return toPointerArray(l.Items)
}

func emptyCheckStatus(t *api.SPIAccessCheck) bool {
	return t.Status.ServiceProvider == ""
}

func newListChecks() *api.SPIAccessCheckList {
	return &api.SPIAccessCheckList{}
}

func listLenChecks(l *api.SPIAccessCheckList) int {
	return len(l.Items)
}

func convertFiles(l *api.SPIFileContentRequestList) []*api.SPIFileContentRequest {
	return toPointerArray(l.Items)
}

func emptyFileStatus(t *api.SPIFileContentRequest) bool {
	return t.Status.Phase == ""
}

func newListFiles() *api.SPIFileContentRequestList {
	return &api.SPIFileContentRequestList{}
}

func listLenFiles(l *api.SPIFileContentRequestList) int {
	return len(l.Items)
}

func convertDataUpdates(l *api.SPIAccessTokenDataUpdateList) []*api.SPIAccessTokenDataUpdate {
	return toPointerArray(l.Items)
}

func emptyDataUpdateStatus(_ *api.SPIAccessTokenDataUpdate) bool {
	// the data updates don't have a status, so we return false here, saying that the "status" is already "filled in".
	return false
}

func newListDataUpdates() *api.SPIAccessTokenDataUpdateList {
	return &api.SPIAccessTokenDataUpdateList{}
}

func listLenDataUpdates(l *api.SPIAccessTokenDataUpdateList) int {
	return len(l.Items)
}

func validateClusterEmpty(g Gomega) {
	checkNoObjectsInCluster(g, newListTokens, listLenTokens)
	checkNoObjectsInCluster(g, newListBindings, listLenBindings)
	checkNoObjectsInCluster(g, newListChecks, listLenChecks)
	checkNoObjectsInCluster(g, newListDataUpdates, listLenDataUpdates)
	checkNoObjectsInCluster(g, newListFiles, listLenFiles)
}

func checkNoObjectsInCluster[L client.ObjectList](g Gomega, list func() L, getLen func(l L) int) {
	l := list()
	g.Expect(ITest.Client.List(ITest.Context, l)).To(Succeed())
	g.Expect(getLen(l)).To(Equal(0))
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

func deleteAll[T client.Object, L client.ObjectList](list func() L, getLen func(list L) int, getItems func(list L) []T) {
	Eventually(func(g Gomega) {
		l := list()
		g.Expect(ITest.Client.List(ITest.Context, l)).To(Succeed())
		for _, o := range getItems(l) {
			g.Expect(ITest.Client.Delete(ITest.Context, o)).To(Succeed())
		}
	}).Should(Succeed())
	Eventually(func(g Gomega) {
		checkNoObjectsInCluster(g, list, getLen)
	}).Should(Succeed())
}

func waitForStatus[T client.Object, L client.ObjectList](list L, itemsFromList func(L) []T, hasEmptyStatus func(T) bool, significantStatusField string) {
	Eventually(func(g Gomega) {
		g.Expect(ITest.Client.List(ITest.Context, list)).To(Succeed())
		items := itemsFromList(list)
		if len(items) == 0 {
			return
		}

		for _, o := range items {
			g.Expect(hasEmptyStatus(o)).To(BeFalse(),
				"%s.%s/%s/%s has empty status.%s (used to detect if the status is empty)",
				strings.ToLower(o.GetObjectKind().GroupVersionKind().Kind),
				o.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				o.GetNamespace(),
				o.GetName(),
				significantStatusField)
		}
	}).Should(Succeed(), "failed to wait for status of objects %T", list)
}

func addAllExisting[T client.Object, L client.ObjectList](list L, itemsFromList func(L) []T) []T {
	var existing []T
	Expect(ITest.Client.List(ITest.Context, list)).To(Succeed())

	for _, o := range itemsFromList(list) {
		existing = append(existing, o)
	}

	return existing
}

func findDifferences[T client.Object](origs []T, news []T) string {
	var diffs []string

	if len(origs) != len(news) {
		return "arrays have a different number of elements"
	}

	origMap := map[client.ObjectKey]T{}
	for _, o := range origs {
		origMap[client.ObjectKeyFromObject(o)] = o
	}

	for _, n := range news {
		key := client.ObjectKeyFromObject(n)
		o := origMap[key]
		diff := diff(o, n)
		if len(diff) > 0 {
			diffs = append(diffs, fmt.Sprintf("%v: {%s}", key, diff))
		}
	}

	return strings.Join(diffs, ", ")
}

func diff[T client.Object](a T, b T) string {
	copyA := a.DeepCopyObject().(T)
	copyB := b.DeepCopyObject().(T)

	// remove the metadata from the objects
	copyA.SetAnnotations(nil)
	copyA.SetResourceVersion("")
	copyA.SetManagedFields(nil)
	copyB.SetAnnotations(nil)
	copyB.SetResourceVersion("")
	copyB.SetManagedFields(nil)

	return strings.Join(deep.Equal(copyA, copyB), ", ")
}

func toPointerArray[T any](arr []T) []*T {
	ret := make([]*T, 0, len(arr))
	for i := range arr {
		// we cannot use the second var from the range assignment here, because that is a single memory location for all
		// elements. We'd therefore end up with an array of pointers to a single memory location.
		ret = append(ret, &arr[i])
	}

	return ret
}

func fromPointerArray[T any](arr []*T) []T {
	ret := make([]T, 0, len(arr))
	for _, o := range arr {
		// we're fine using "o" here as opposed to the situation in toPointerArray, because here we're effectively
		// creating a copy of the object
		ret = append(ret, *o)
	}

	return ret
}
