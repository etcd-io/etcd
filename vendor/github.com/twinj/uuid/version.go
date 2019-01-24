package uuid

// Version represents the type of UUID.
type Version int

// The following are the supported Versions.
const (
	VersionUnknown Version = iota // Unknown
	VersionOne                    // Time based
	VersionTwo                    // DCE security via POSIX UIDs
	VersionThree                  // Namespace hash uses MD5
	VersionFour                   // Crypto random
	VersionFive                   // Namespace hash uses SHA-1
)

// The following are the supported Variants.
const (
	VariantNCS       uint8 = 0x00
	VariantRFC4122   uint8 = 0x80 // or and A0 if masked with 1F
	VariantMicrosoft uint8 = 0xC0
	VariantFuture    uint8 = 0xE0
)

const (
	// 3f used by RFC4122 although 1f works for all
	variantSet = 0x3f

	// rather than using 0xc0 we use 0xe0 to retrieve the variant
	// The result is the same for all other variants
	// 0x80 and 0xa0 are used to identify RFC4122 compliance
	variantGet = 0xe0
)

// String returns English description of version.
func (o Version) String() string {
	switch o {
	case VersionOne:
		return "Version 1: Based on a 60 Bit Timestamp."
	case VersionTwo:
		return "Version 2: Based on DCE security domain and 60 bit timestamp."
	case VersionThree:
		return "Version 3: Namespace UUID and unique names hashed by MD5."
	case VersionFour:
		return "Version 4: Crypto-random generated."
	case VersionFive:
		return "Version 5: Namespace UUID and unique names hashed by SHA-1."
	default:
		return "Unknown: Not supported"
	}
}

func resolveVersion(version uint8) Version {
	switch Version(version) {
	case VersionOne, VersionTwo, VersionThree, VersionFour, VersionFive:
		return Version(version)
	default:
		return VersionUnknown
	}
}

func variant(variant uint8) uint8 {
	switch variant & variantGet {
	case VariantRFC4122, 0xA0:
		return VariantRFC4122
	case VariantMicrosoft:
		return VariantMicrosoft
	case VariantFuture:
		return VariantFuture
	}
	return VariantNCS
}
