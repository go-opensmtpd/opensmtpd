package opensmtpd

func ExampleTable() {
	// In smtpd.conf:
	//
	//   table aliases <name-of-filter>:
	//   accept for local alias <aliases> ...

	aliases := map[string]string{
		"root": "user@example.org",
	}

	table := &Table{
		Lookup: func(service int, params Dict, key string) (string, error) {
			// We are only valid for aliases
			if service&ServiceAlias != 0 {
				return aliases[key], nil
			}
			return "", nil
		},
	}
	table.Serve()
}
