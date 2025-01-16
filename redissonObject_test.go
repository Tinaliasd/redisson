package redisson

import "testing"

func TestPrefixName(t *testing.T) {
	o := newRedissonObjectNULL("test")
	name := o.prefixName("prefix", "{redisson}")
	if name != "prefix:{redisson}" {
		t.Fatal(name)
	}
	name = o.prefixName("prefix", "redisson")
	if name != "prefix:{redisson}" {
		t.Fatal(name)
	}
}

func TestSuffixName(t *testing.T) {
	o := newRedissonObjectNULL("test")
	name := o.suffixName("{redisson}", "suffix")
	if name != "{redisson}:suffix" {
		t.Fatal(name)
	}
	name = o.suffixName("redisson", "suffix")
	if name != "{redisson}:suffix" {
		t.Fatal(name)
	}
}
