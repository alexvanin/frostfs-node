package engine

import (
	"os"
	"testing"

	"github.com/TrueCloudLab/frostfs-node/pkg/core/object"
	"github.com/TrueCloudLab/frostfs-node/pkg/local_object_storage/shard"
	cidtest "github.com/TrueCloudLab/frostfs-sdk-go/container/id/test"
	objectSDK "github.com/TrueCloudLab/frostfs-sdk-go/object"
	"github.com/stretchr/testify/require"
)

func TestStorageEngine_Inhume(t *testing.T) {
	defer os.RemoveAll(t.Name())

	cnr := cidtest.ID()
	splitID := objectSDK.NewSplitID()

	fs := objectSDK.SearchFilters{}
	fs.AddRootFilter()

	tombstoneID := object.AddressOf(generateObjectWithCID(t, cnr))
	parent := generateObjectWithCID(t, cnr)

	child := generateObjectWithCID(t, cnr)
	child.SetParent(parent)
	idParent, _ := parent.ID()
	child.SetParentID(idParent)
	child.SetSplitID(splitID)

	link := generateObjectWithCID(t, cnr)
	link.SetParent(parent)
	link.SetParentID(idParent)
	idChild, _ := child.ID()
	link.SetChildren(idChild)
	link.SetSplitID(splitID)

	t.Run("delete small object", func(t *testing.T) {
		e := testNewEngineWithShardNum(t, 1)
		defer e.Close()

		err := Put(e, parent)
		require.NoError(t, err)

		var inhumePrm InhumePrm
		inhumePrm.WithTarget(tombstoneID, object.AddressOf(parent))

		_, err = e.Inhume(inhumePrm)
		require.NoError(t, err)

		addrs, err := Select(e, cnr, fs)
		require.NoError(t, err)
		require.Empty(t, addrs)
	})

	t.Run("delete big object", func(t *testing.T) {
		s1 := testNewShard(t, 1)
		s2 := testNewShard(t, 2)

		e := testNewEngineWithShards(s1, s2)
		defer e.Close()

		var putChild shard.PutPrm
		putChild.SetObject(child)
		_, err := s1.Put(putChild)
		require.NoError(t, err)

		var putLink shard.PutPrm
		putLink.SetObject(link)
		_, err = s2.Put(putLink)
		require.NoError(t, err)

		var inhumePrm InhumePrm
		inhumePrm.WithTarget(tombstoneID, object.AddressOf(parent))

		_, err = e.Inhume(inhumePrm)
		require.NoError(t, err)

		addrs, err := Select(e, cnr, fs)
		require.NoError(t, err)
		require.Empty(t, addrs)
	})
}
