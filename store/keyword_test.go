package store

import (
	"testing"
)

func TestKeywords(t *testing.T) {
	keyword := CheckKeyword("machines")
	if !keyword {
		t.Fatal("machines should be keyword")
	}

	keyword = CheckKeyword("/machines")

	if !keyword {
		t.Fatal("/machines should be keyword")
	}

	keyword = CheckKeyword("/machines/")

	if !keyword {
		t.Fatal("/machines/ contains keyword prefix")
	}

	keyword = CheckKeyword("/machines/node1")

	if !keyword {
		t.Fatal("/machines/* contains keyword prefix")
	}

	keyword = CheckKeyword("/nokeyword/machines/node1")

	if keyword {
		t.Fatal("this does not contain keyword prefix")
	}

}
