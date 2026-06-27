package cwe

const (
	// Acronym is the acronym of CWE
	Acronym = "CWE"
	// Version the CWE version
	Version = "4.4"
	// ReleaseDateUtc the release Date of CWE Version
	ReleaseDateUtc = "2021-03-15"
	// Organization MITRE
	Organization = "MITRE"
	// Description the description of CWE
	Description = "The MITRE Common Weakness Enumeration"
	// InformationURI link to the published CWE PDF
	InformationURI = "https://cwe.mitre.org/data/published/cwe_v" + Version + ".pdf/"
	// DownloadURI link to the zipped XML of the CWE list
	DownloadURI = "https://cwe.mitre.org/data/xml/cwec_v" + Version + ".xml.zip"
)

var idWeaknesses = map[string]*Weakness{
	"22": {
		ID:          "22",
		Description: "The software uses external input to construct a pathname that is intended to identify a file or directory that is located underneath a restricted parent directory, but the software does not properly neutralize special elements within the pathname that can cause the pathname to resolve to a location that is outside of the restricted directory.",
		Name:        "Improper Limitation of a Pathname to a Restricted Directory ('Path Traversal')",
	},
	"78": {
		ID:          "78",
		Description: "The software constructs all or part of an OS command using externally-influenced input from an upstream component, but it does not neutralize or incorrectly neutralizes special elements that could modify the intended OS command when it is sent to a downstream component.",
		Name:        "Improper Neutralization of Special Elements used in an OS Command ('OS Command Injection')",
	},
	"79": {
		ID:          "79",
		Description: "The software does not neutralize or incorrectly neutralizes user-controllable input before it is placed in output that is used as a web page that is served to other users.",
		Name:        "Improper Neutralization of Input During Web Page Generation ('Cross-site Scripting')",
	},
	"88": {
		ID:          "88",
		Description: "The software constructs a string for a command to executed by a separate component\nin another control sphere, but it does not properly delimit the\nintended arguments, options, or switches within that command string.",
		Name:        "Improper Neutralization of Argument Delimiters in a Command ('Argument Injection')",
	},
	"89": {
		ID:          "89",
		Description: "The software constructs all or part of an SQL command using externally-influenced input from an upstream component, but it does not neutralize or incorrectly neutralizes special elements that could modify the intended SQL command when it is sent to a downstream component.",
		Name:        "Improper Neutralization of Special Elements used in an SQL Command ('SQL Injection')",
	},
	"93": {
		ID:          "93",
		Description: "The software does not properly neutralize CRLF sequences before using externally-influenced input in protocol elements that rely on CRLF as delimiters, allowing attackers to inject additional commands or headers.",
		Name:        "Improper Neutralization of CRLF Sequences ('CRLF Injection')",
	},
	"94": {
		ID:          "94",
		Description: "The software constructs all or part of a code segment using externally-influenced input from an upstream component, but it does not neutralize or incorrectly neutralizes special elements that could modify the syntax or behavior of the intended code segment.",
		Name:        "Improper Control of Generation of Code ('Code Injection')",
	},
	"118": {
		ID:          "118",
		Description: "The software does not restrict or incorrectly restricts operations within the boundaries of a resource that is accessed using an index or pointer, such as memory or files.",
		Name:        "Incorrect Access of Indexable Resource ('Range Error')",
	},
	"190": {
		ID:          "190",
		Description: "The software performs a calculation that can produce an integer overflow or wraparound, when the logic assumes that the resulting value will always be larger than the original value. This can introduce other weaknesses when the calculation is used for resource management or execution control.",
		Name:        "Integer Overflow or Wraparound",
	},
	"200": {
		ID:          "200",
		Description: "The product exposes sensitive information to an actor that is not explicitly authorized to have access to that information.",
		Name:        "Exposure of Sensitive Information to an Unauthorized Actor",
	},
	"242": {
		ID:          "242",
		Description: "The program calls a function that can never be guaranteed to work safely.",
		Name:        "Use of Inherently Dangerous Function",
	},
	"276": {
		ID:          "276",
		Description: "During installation, installed file permissions are set to allow anyone to modify those files.",
		Name:        "Incorrect Default Permissions",
	},
	"287": {
		ID:          "287",
		Description: "The software does not perform or incorrectly performs authentication.",
		Name:        "Improper Authentication",
	},
	"295": {
		ID:          "295",
		Description: "The software does not validate, or incorrectly validates, a certificate.",
		Name:        "Improper Certificate Validation",
	},
	"310": {
		ID:          "310",
		Description: "Weaknesses in this category are related to the design and implementation of data confidentiality and integrity. Frequently these deal with the use of encoding techniques, encryption libraries, and hashing algorithms. The weaknesses in this category could lead to a degradation of the quality data if they are not addressed.",
		Name:        "Cryptographic Issues",
	},
	"322": {
		ID:          "322",
		Description: "The software performs a key exchange with an actor without verifying the identity of that actor.",
		Name:        "Key Exchange without Entity Authentication",
	},
	"326": {
		ID:          "326",
		Description: "The software stores or transmits sensitive data using an encryption scheme that is theoretically sound, but is not strong enough for the level of protection required.",
		Name:        "Inadequate Encryption Strength",
	},
	"327": {
		ID:          "327",
		Description: "The use of a broken or risky cryptographic algorithm is an unnecessary risk that may result in the exposure of sensitive information.",
		Name:        "Use of a Broken or Risky Cryptographic Algorithm",
	},
	"328": {
		ID:          "328",
		Description: "The product uses an algorithm that produces a digest (output value) that does not meet security expectations for a hash function that allows an adversary to reasonably determine the original input (preimage attack), find another input that can produce the same hash (2nd preimage attack), or find multiple inputs that evaluate to the same hash (birthday attack). ",
		Name:        "Use of Weak Hash",
	},
	"338": {
		ID:          "338",
		Description: "The product uses a Pseudo-Random Number Generator (PRNG) in a security context, but the PRNG's algorithm is not cryptographically strong.",
		Name:        "Use of Cryptographically Weak Pseudo-Random Number Generator (PRNG)",
	},
	"367": {
		ID:          "367",
		Description: "The software checks the state of a resource before using that resource, but the resource's state can change between the check and the use in a way that invalidates the results of the check.",
		Name:        "Time-of-check Time-of-use (TOCTOU) Race Condition",
	},
	"377": {
		ID:          "377",
		Description: "Creating and using insecure temporary files can leave application and system data vulnerable to attack.",
		Name:        "Insecure Temporary File",
	},
	"400": {
		ID:          "400",
		Description: "The software does not properly control the allocation and maintenance of a limited resource, thereby enabling an actor to influence the amount of resources consumed, eventually leading to the exhaustion of available resources.",
		Name:        "Uncontrolled Resource Consumption",
	},
	"409": {
		ID:          "409",
		Description: "The software does not handle or incorrectly handles a compressed input with a very high compression ratio that produces a large output.",
		Name:        "Improper Handling of Highly Compressed Data (Data Amplification)",
	},
	"444": {
		ID:          "444",
		Description: "When malformed or unexpected HTTP requests are inconsistently interpreted by one or more entities in the data flow between the user and the web server, such as a proxy or firewall, attackers can abuse this discrepancy to smuggle requests to one system without the other system being aware of it.",
		Name:        "Inconsistent Interpretation of HTTP Requests ('HTTP Request Smuggling')",
	},
	"499": {
		ID:          "499",
		Description: "The code contains a class with sensitive data, but the class does not explicitly deny serialization. The data can be accessed by serializing the class through another class.",
		Name:        "Serializable Class Containing Sensitive Data",
	},
	"676": {
		ID:          "676",
		Description: "The program invokes a potentially dangerous function that could introduce a vulnerability if it is used incorrectly, but the function can also be used safely.",
		Name:        "Use of Potentially Dangerous Function",
	},
	"703": {
		ID:          "703",
		Description: "The software does not properly anticipate or handle exceptional conditions that rarely occur during normal operation of the software.",
		Name:        "Improper Check or Handling of Exceptional Conditions",
	},
	"798": {
		ID:          "798",
		Description: "The software contains hard-coded credentials, such as a password or cryptographic key, which it uses for its own inbound authentication, outbound communication to external components, or encryption of internal data.",
		Name:        "Use of Hard-coded Credentials",
	},
	"1204": {
		ID:          "1204",
		Description: "The product uses a cryptographic primitive that uses an Initialization Vector (IV), but the product does not generate IVs that are sufficiently unpredictable or unique according to the expected cryptographic requirements for that primitive.",
		Name:        "Generation of Weak Initialization Vector (IV)",
	},
	"117": {
		ID:          "117",
		Description: "The software does not neutralize or incorrectly neutralizes output that is written to logs.",
		Name:        "Improper Output Neutralization for Logs",
	},
	"502": {
		ID:          "502",
		Description: "The application deserializes untrusted data without sufficiently verifying that the resulting data will be valid.",
		Name:        "Deserialization of Untrusted Data",
	},
	"614": {
		ID:          "614",
		Description: "The Secure attribute for a sensitive cookie is not set, which could cause the user agent to send that cookie in plaintext over an HTTP session.",
		Name:        "Sensitive Cookie in HTTPS Session Without 'Secure' Attribute",
	},
	"918": {
		ID:          "918",
		Description: "The web server receives a URL or similar request from an upstream component and retrieves the contents of this URL, but it does not sufficiently ensure that the request is being sent to the expected destination.",
		Name:        "Server-Side Request Forgery (SSRF)",
	},
}

// Get Retrieves a CWE weakness by it's id
func Get(id string) *Weakness {
	weakness, ok := idWeaknesses[id]
	if ok && weakness != nil {
		return weakness
	}
	return nil
}
