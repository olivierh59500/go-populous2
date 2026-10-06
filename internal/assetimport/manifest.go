// Package assetimport prepares game data from user-supplied Amiga files.
package assetimport

type Fingerprint struct {
	Name   string
	Size   int
	SHA256 string
}

// Files identifies the supported original revision without storing its bytes.
var Files = []Fingerprint{
	{Name: "awards.pak", Size: 5696, SHA256: "f2c1cccece531b0d312bc4f0e2d703899c8607769193c9cf645a84e24711386f"},
	{Name: "block0.pak", Size: 27372, SHA256: "eb97d9dde82f19cf7c72d5b927545eb5a94e340c9fefd530cb3190d6d3ee3225"},
	{Name: "block1.pak", Size: 26248, SHA256: "4ecc497361e30f35c443521df4942f6d7e2ea8167bbf2066fb9adc19b91709ee"},
	{Name: "block2.pak", Size: 27120, SHA256: "a75e8376ac852cc1955a2f656d4c72f7cbc437b16629f3648210daaf2f344dc3"},
	{Name: "block3.pak", Size: 27588, SHA256: "6e567154295b9dc185efb6ea920dd6433694b882d69e82d77cd5343be85389ee"},
	{Name: "conquest.pak", Size: 11324, SHA256: "91096c4b65f5afa8fd0b927294d26e76749154c4b659c1165358dde0dbbe6f7f"},
	{Name: "end.pak", Size: 115840, SHA256: "106cccea75ee5d711107c26b4a393ccebb0eb8e22441e744b0533de02a3d12de"},
	{Name: "faces.pak", Size: 6644, SHA256: "84a21f47cb308e044bc4832fd555a2e4b1671a79bf0685f13b84f9a823584038"},
	{Name: "fx.dat", Size: 123100, SHA256: "76ecba2baec55973d37497605e65c38849c7c95ed61527a8dd23c2174859f495"},
	{Name: "hands.pak", Size: 6116, SHA256: "35d33de4734d1790bc196b0c19fcf75aa1efc4c352d3bdefdf65c6af160a3830"},
	{Name: "judge.pak", Size: 36412, SHA256: "7bfd5cc30421f0bbb38553d22e323771441348f55dbb83fa54ace1196a6bf617"},
	{Name: "land0.dat", Size: 556, SHA256: "48af5150c8a9cc731057dfb4855df0564547119ec7bd066df34faee087e73098"},
	{Name: "land1.dat", Size: 556, SHA256: "44ca300b24087300f1c58b1de731011d73e647ac54d1de4f07ddeb8c39841377"},
	{Name: "land2.dat", Size: 556, SHA256: "4faa952687368981bebbbb1a1ef0840b414d92c9ca127ed366c152f0a13da51a"},
	{Name: "land3.dat", Size: 556, SHA256: "dd2abf5d54f3999a324aabc5dc700bd37d837c0da2cef64605771e392ada9bdf"},
	{Name: "populous.ii", Size: 297996, SHA256: "c148b9bdcc543d89bf788d86886eb5ac9320ad66b09ef92792ec24314bfee803"},
	{Name: "qaz.pak", Size: 16988, SHA256: "91ad87a19054c10cf7e45cf6a4ae2c6976ff18973b0947ff08931dc2b06aa316"},
	{Name: "s16-0.dif", Size: 10, SHA256: "0083af118d18a63c6bb552f21d0c4ee78741f988ecd319d3cd06cb6c85a68a63"},
	{Name: "s16-0.pak", Size: 55568, SHA256: "a66812eddee231b938bf8605da2b41d62b614657ad3c6f3440401fb14b0cf6d5"},
	{Name: "s16-1.dif", Size: 10, SHA256: "0083af118d18a63c6bb552f21d0c4ee78741f988ecd319d3cd06cb6c85a68a63"},
	{Name: "s16-2.dif", Size: 10, SHA256: "0083af118d18a63c6bb552f21d0c4ee78741f988ecd319d3cd06cb6c85a68a63"},
	{Name: "s16-3.dif", Size: 10, SHA256: "0083af118d18a63c6bb552f21d0c4ee78741f988ecd319d3cd06cb6c85a68a63"},
	{Name: "s32-0.dif", Size: 10, SHA256: "0083af118d18a63c6bb552f21d0c4ee78741f988ecd319d3cd06cb6c85a68a63"},
	{Name: "s32-0.pak", Size: 47044, SHA256: "293467f0ba4d3d3a2f64038f62339278202525bafecb5b7712a200a54ffc6ce1"},
	{Name: "s32-1.pif", Size: 18780, SHA256: "870ded809f2e3f0cd331f11e0809a71486c9679b952cde4597457210edcb2ca7"},
	{Name: "s32-2.pif", Size: 8364, SHA256: "21efb7d8db23b3dfe871d83a3eec17675628534d758fdb98b81fbf82e7a7e17e"},
	{Name: "s32-3.pif", Size: 13888, SHA256: "07f305580bb919e7d03c65c4ee1314e5f06c1705e24e30b6bf44b844aebbf7aa"},
}
