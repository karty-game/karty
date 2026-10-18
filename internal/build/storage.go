package build

func validStorageIdentity(identity string) bool {
	if len(identity) == 0 || len(identity) > 256 {
		return false
	}

	for _, character := range identity {
		alphanumeric := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		separator := character == '.' || character == '/' || character == '_' || character == '-'

		if !alphanumeric && !separator {
			return false
		}
	}

	return true
}
