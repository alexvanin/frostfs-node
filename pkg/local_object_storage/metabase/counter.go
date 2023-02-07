package meta

import (
	"encoding/binary"
	"fmt"

	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	oid "github.com/TrueCloudLab/frostfs-sdk-go/object/id"
	"go.etcd.io/bbolt"
)

var objectPhyCounterKey = []byte("phy_counter")
var objectLogicCounterKey = []byte("logic_counter")

type objectType uint8

const (
	_ objectType = iota
	phy
	logical
)

// ObjectCounters groups object counter
// according to metabase state.
type ObjectCounters struct {
	logic uint64
	phy   uint64
}

// Logic returns logical object counter.
func (o ObjectCounters) Logic() uint64 {
	return o.logic
}

// Phy returns physical object counter.
func (o ObjectCounters) Phy() uint64 {
	return o.phy
}

// ObjectCounters returns object counters that metabase has
// tracked since it was opened and initialized.
//
// Returns only the errors that do not allow reading counter
// in Bolt database.
func (db *DB) ObjectCounters() (cc ObjectCounters, err error) {
	db.modeMtx.RLock()
	defer db.modeMtx.RUnlock()

	if db.mode.NoMetabase() {
		return ObjectCounters{}, ErrDegradedMode
	}

	err = db.boltDB.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(shardInfoBucket)
		if b != nil {
			data := b.Get(objectPhyCounterKey)
			fmt.Println("!! phy key size", len(data))
			if len(data) == 8 {
				cc.phy = binary.LittleEndian.Uint64(data)
			}

			fmt.Println("!! log key size", len(data))
			data = b.Get(objectLogicCounterKey)
			if len(data) == 8 {
				cc.logic = binary.LittleEndian.Uint64(data)
			}
		} else {
			fmt.Println("!!! bucket does not exist")
		}

		return nil
	})

	return
}

// updateCounter updates the object counter. Tx MUST be writable.
// If inc == `true`, increases the counter, decreases otherwise.
func (db *DB) updateCounter(tx *bbolt.Tx, typ objectType, delta uint64, inc bool) error {
	fmt.Println("!! I update metabase counter")
	b := tx.Bucket(shardInfoBucket)
	if b == nil {
		fmt.Println("!! shard info bucket is not available")
		return nil
	}

	var counter uint64
	var counterKey []byte

	switch typ {
	case phy:
		counterKey = objectPhyCounterKey
	case logical:
		counterKey = objectLogicCounterKey
	default:
		panic("unknown object type counter")
	}

	data := b.Get(counterKey)
	if len(data) == 8 {
		counter = binary.LittleEndian.Uint64(data)
	}

	if inc {
		counter += delta
	} else if counter <= delta {
		counter = 0
	} else {
		counter -= delta
	}

	newCounter := make([]byte, 8)
	binary.LittleEndian.PutUint64(newCounter, counter)

	fmt.Println("!! new counter value", counter, string(counterKey))

	return b.Put(counterKey, newCounter)
}

// syncCounter updates object counters according to metabase state:
// it counts all the physically/logically stored objects using internal
// indexes. Tx MUST be writable.
//
// Does nothing if counters are not empty and force is false. If force is
// true, updates the counters anyway.
func syncCounter(tx *bbolt.Tx, force bool) error {
	b, err := tx.CreateBucketIfNotExists(shardInfoBucket)
	if err != nil {
		return fmt.Errorf("could not get shard info bucket: %w", err)
	}

	if !force && len(b.Get(objectPhyCounterKey)) == 8 && len(b.Get(objectLogicCounterKey)) == 8 {
		// the counters are already inited
		return nil
	}

	var addr oid.Address
	var phyCounter uint64
	var logicCounter uint64

	graveyardBKT := tx.Bucket(graveyardBucketName)
	garbageBKT := tx.Bucket(garbageBucketName)
	key := make([]byte, addressKeySize)

	err = iteratePhyObjects(tx, func(cnr cid.ID, obj oid.ID) error {
		phyCounter++

		addr.SetContainer(cnr)
		addr.SetObject(obj)

		// check if an object is available: not with GCMark
		// and not covered with a tombstone
		if inGraveyardWithKey(addressKey(addr, key), graveyardBKT, garbageBKT) == 0 {
			logicCounter++
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not iterate objects: %w", err)
	}

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, phyCounter)

	err = b.Put(objectPhyCounterKey, data)
	if err != nil {
		return fmt.Errorf("could not update phy object counter: %w", err)
	}

	data = make([]byte, 8)
	binary.LittleEndian.PutUint64(data, logicCounter)

	err = b.Put(objectLogicCounterKey, data)
	if err != nil {
		return fmt.Errorf("could not update logic object counter: %w", err)
	}

	return nil
}
