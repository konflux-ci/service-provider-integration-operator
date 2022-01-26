package serviceprovider

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	stateLabel       = "spi.appstudio.redhat.com/sp-state-for-token"
	stateKindLabel   = "spi.appstudio.redhat.com/sp-state-kind"
	lastRefreshField = "lastRefresh"
	dataField        = "data"
)

var (
	secretDiff = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}
)

// StateCache acts like a cache of serialized state of tokens. It actually uses Kubernetes secrets as the storage
// mechanism (and therefore assumes caching at the kubernetes client level). This cache assigns a TTL to the data.
type StateCache struct {
	// Kind describes the kind of data being cached
	Kind string
	// Ttl limits how long the data stays in the cache
	Ttl time.Duration

	client client.Client
	sync   sync.Syncer
}

// NewStateCache creates a new cache instance with the provided configuration.
func NewStateCache(ttl time.Duration, kind string, client client.Client) StateCache {
	return StateCache{
		Kind:   kind,
		Ttl:    ttl,
		client: client,
		sync:   sync.New(client),
	}
}

// Put stores the provided token state into the cache.
func (c *StateCache) Put(ctx context.Context, token *api.SPIAccessToken, data []byte) error {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "sp-token-state-" + c.Kind,
			Labels: map[string]string{
				stateLabel:     token.Name,
				stateKindLabel: c.Kind,
			},
		},
	}

	refreshAsBytes := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(refreshAsBytes, time.Now().Unix())

	s.Data[dataField] = data
	s.Data[lastRefreshField] = refreshAsBytes

	_, _, err := c.sync.Sync(ctx, token, s, secretDiff)
	return err
}

func (c *StateCache) Get(ctx context.Context, token *api.SPIAccessToken) ([]byte, error) {
	ss := &corev1.SecretList{}
	if err := c.client.List(ctx, ss, client.InNamespace(token.Namespace), client.MatchingLabels{
		stateLabel:     token.Name,
		stateKindLabel: c.Kind,
	}); err != nil {
		return nil, err
	}

	var s *corev1.Secret

	if len(ss.Items) > 0 {
		s = &ss.Items[0]
		timeStamp, ok := binary.Varint(s.Data[lastRefreshField])
		if ok <= 0 {
			return nil, fmt.Errorf("failed to read the last refresh timestamp from the serialized state of token %s in namespace %s", token.Name, token.Namespace)
		}
		lastRefresh := time.Unix(timeStamp, 0)

		now := time.Now()
		if now.Before(lastRefresh.Add(c.Ttl)) {
			// the cached data is still considered valid
			return s.Data[dataField], nil
		}
	}

	return nil, nil
}

// Ensure combines Get and Put. Returns the cached data if it is valid, or uses the provided serializer to serialize
// the token state, caches it and returns the freshly obtained data.
func (c *StateCache) Ensure(ctx context.Context, token *api.SPIAccessToken, ser StateSerializer) ([]byte, error) {
	state, err := c.Get(ctx, token)
	if err != nil {
		return nil, err
	}

	if state == nil {
		state, err = ser.Serialize(ctx, token)
		if err != nil {
			return nil, err
		}
		err = c.Put(ctx, token, state)
		if err != nil {
			return nil, err
		}
	}

	return state, nil
}
