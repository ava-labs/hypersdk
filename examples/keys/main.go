package main

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

// hardcoded initial set of ed25519 keys. Each will be initialized with InitialBalance
var ed25519HexKeys = []string{
	"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
	"D4E950BC5147FD45F612BE8793F4C38EEA0710EF8BBB335756DAAA9D605CD64ED2E74C5DF3B0529C1C274E5C8023D5B000200F5C542C82E29CC7ADA503B87C0C", //nolint:lll
	"EDA13E9797EBD181FF51C03F383BC77FC79C2489C35BE66878C01E0EDA824710F9130058A5DA232010CDA22C9F79C52BDAB59745D9E9FDC3C7848A5964FE2B94",
	"47A785559736053C9F7F6352BE2C1330AA5EB53A3E73D6B486D03086768FCE1C30217F1E3A3BE88DD9BD1B27FDD3FD0C4AA0790FAAF7DA79894625609A3AA0FC",
}


func main() {
	testKeys := make([]ed25519.PrivateKey, len(ed25519HexKeys))
	for i, keyHex := range ed25519HexKeys {
		bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		testKeys[i] = ed25519.PrivateKey(bytes)
	}

	for _, key := range testKeys {
		address := auth.NewED25519Address(key.PublicKey())
		println(address.String())
	}
}

// output 
// 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d914969017
// 0x003020338128fc7babb4e5850aace96e589f55b33bda90d62c44651de110ea5b8c0b5ee37f
// 0x006cf906f2df7c34d9be247dd384aefb43510d37d6c9ab273199d68c3b85130bcd05081a2c
// 0x00f45cd0197c148ebd317efe15ef91df1af7b5092f01f6e36cd4063c2e6ee149403f724a12
// 0x00e3ae18f245e3bbb50860db2f44a0fe460e1de92b698d6370ec3112501ec6d9ba26a9d456