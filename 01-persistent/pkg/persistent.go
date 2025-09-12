package persistent

import (
	"os"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/examples"
	dssync "github.com/ipfs/go-datastore/sync"
	badgerds "github.com/ipfs/go-ds-badger"
	pebbleds "github.com/ipfs/go-ds-pebble"

	block "github.com/gosunuts/boxo-starter-kit/00-block-cid/pkg"
)

type PersistentType string

const (
	Memory   PersistentType = "memory"
	File     PersistentType = "file"
	Badgerdb PersistentType = "badgerdb"
	Pebbledb PersistentType = "pebbledb"
)

type PersistentWrapper struct {
	batching ds.Batching
	*block.BlockWrapper
}

func New(ptype PersistentType, path string) (*PersistentWrapper, error) {
	if path == "" {
		path = os.TempDir() + string(ptype)
	}

	var batching ds.Batching
	var err error
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	switch ptype {
	case Memory:
		batching = dssync.MutexWrap(ds.NewMapDatastore())
	case File:
		datastore, err := examples.NewDatastore(path)
		if err != nil {
			return nil, err
		}
		batching = datastore.(*examples.Datastore)
	case Badgerdb:
		batching, err = badgerds.NewDatastore(path, nil)
		if err != nil {
			return nil, err
		}
	case Pebbledb:
		batching, err = pebbleds.NewDatastore(path, nil)
		if err != nil {
			return nil, err
		}
	}
	blockWrapper := block.New(batching)

	return &PersistentWrapper{
		batching:     batching,
		BlockWrapper: blockWrapper,
	}, nil
}

func (p *PersistentWrapper) Close() error {
	return p.batching.Close()
}
