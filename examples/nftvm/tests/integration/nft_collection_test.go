// Tests core logic of NFT Collections and Instances

package integration_test

import "github.com/ava-labs/hypersdk/codec"

const (
	CollectionNameOne = "The Louvre Museum"
	CollectionSymbolOne = "LVM"
	CollectionMetadataOne = "The most famous museum in the world"

	InstanceMetadataOne = "Mona Lisa by Leonardo Da Vinci"
)

var (
	firstCollectionAddress codec.Address
	readableFirstCollectionAddress string
	firstInstanceNum uint32 = 0
)