package store

import (
	"testing"
)

func TestKeywords(t *testing.T) {
	keyword := CheckKeyword("_etcd")
	if !keyword {
		t.Fatal("_etcd should be keyword")
	}

	keyword = CheckKeyword("/_etcd")

	if !keyword {
		t.Fatal("/_etcd should be keyword")
	}

	keyword = CheckKeyword("/_etcd/")

	if !keyword {
		t.Fatal("/_etcd/ contains keyword prefix")
	}

	keyword = CheckKeyword("/_etcd/node1")

	if !keyword {
		t.Fatal("/_etcd/* contains keyword prefix")
	}

	keyword = CheckKeyword("/nokeyword/_etcd/node1")

	if keyword {
		t.Fatal("this does not contain keyword prefix")
	}

}
