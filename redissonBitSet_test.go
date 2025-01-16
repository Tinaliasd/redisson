package redisson

import "testing"

func TestSetUnsigned(t *testing.T) {
	bs := GetRedisson().GetBitSet("testUnsigned1")
	if v, err := bs.setUnSigned(8, 1, 120); err != nil {
		panic(err)
	} else if v != 0 {
		panic("err")
	}
	if v, err := bs.incrementAndGetUnSigned(8, 1, 1); err != nil {
		panic(err)
	} else if v != 121 {
		panic("err")
	}

	if v, err := bs.getUnsigned(8, 1); err != nil {
		panic(err)
	} else if v != 121 {
		panic("err")
	}
}

func TestSigned(t *testing.T) {
	bs := GetRedisson().GetBitSet("testSigned")
	if v, err := bs.setSigned(8, 1, -120); err != nil {
		panic(err)
	} else if v != 0 {
		panic("err")
	}
	if v, err := bs.incrementAndGetSigned(8, 1, 1); err != nil {
		panic(err)
	} else if v != -119 {
		panic("err")
	}

	if v, err := bs.getSigned(8, 1); err != nil {
		panic(err)
	} else if v != -119 {
		panic("err")
	}
}

func TestIncrement(t *testing.T) {
	bs := GetRedisson().GetBitSet("testBitSet2")
	if r, err := bs.SetByte(2, byte(12)); err != nil {
		panic(err)
	} else if r != byte(0) {
		panic("err")
	}
	if r, err := bs.GetByte(2); err != nil {
		panic(err)
	} else if r != byte(12) {
		panic("err")
	}

	if r, err := bs.incrementAndGetByte(2, byte(12)); err != nil {
		panic(err)

	} else if r != byte(24) {
		panic("err")

	}
	if r, err := bs.GetByte(2); err != nil {
		panic(err)
	} else if r != byte(24) {
		panic("err")

	}
}

func TestSetGetNumber(t *testing.T) {
	bs := GetRedisson().GetBitSet("testbitset7")
	if r, err := bs.SetInt64(2, 12); err != nil {
		panic(err)
	} else if r != 0 {
		panic("err")
	}
	if r, err := bs.GetInt64(2); err != nil {
		panic(err)
	} else if r != 12 {
		panic("err")
	}

	bs = GetRedisson().GetBitSet("testbitset8")
	if r, err := bs.SetByte(2, byte(12)); err != nil {
		panic(err)
	} else if r != byte(0) {
		panic("err")
	}
	if r, err := bs.GetByte(2); err != nil {
		panic(err)
	} else if r != byte(12) {
		panic("err")
	}

	bs = GetRedisson().GetBitSet("testbitset9")
	if r, err := bs.SetShort(2, 16); err != nil {
		panic(err)
	} else if r != 0 {
		panic("err")
	}
	if r, err := bs.GetShort(2); err != nil {
		panic(err)
	} else if r != 16 {
		panic("err")
	}

	bs = GetRedisson().GetBitSet("testbitset10")
	if r, err := bs.SetInt32(2, 23513); err != nil {
		panic(err)
	} else if r != 0 {
		panic("err")
	}
	if r, err := bs.GetInt32(2); err != nil {
		panic(err)
	} else if r != 23513 {
		panic("err")
	}

}
