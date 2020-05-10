package invalidtest

import "os"

func Not(this *os.File) string { return "the test file is garbage" }
