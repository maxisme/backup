package utils

import (
	"os"
	"testing"
)

func TestEncryptCompressDir(t *testing.T) {
	inFile := "file.tar.gz.encr"
	outFile := "file.tar.gz"
	key := "90871670990532809087167099053280"
	err := EncryptCompressDir("./test", inFile, key, nil)
	if err != nil {
		t.Error(err.Error())
	}

	err = DecryptTar(inFile, outFile, key)
	if err != nil {
		t.Error(err.Error())
	}
	os.RemoveAll(inFile)
	os.RemoveAll(outFile)
}

func TestExcludeDir(t *testing.T) {
	inFile := "file.tar.gz.encr"
	outFile := "file.tar.gz"
	key := "90871670990532809087167099053280"
	err := EncryptCompressDir("./test", inFile, key, []string{"/test/excludeme/*"})
	if err != nil {
		t.Error(err.Error())
	}

	err = DecryptTar(inFile, outFile, key)
	if err != nil {
		t.Error(err.Error())
	}

	fi, _ := os.Stat(outFile)
	// get the size
	size := fi.Size()
	if size < 158-4 || size > 158+4 {
		t.Errorf("%d %d", size, 158)
	}

	os.RemoveAll(inFile)
	os.RemoveAll(outFile)
}

func TestExcludeNestedDir(t *testing.T) {
	inFile := "file.tar.gz.encr"
	outFile := "file.tar.gz"
	key := "90871670990532809087167099053280"
	err := EncryptCompressDir("./test", inFile, key, []string{"/test/excludeme/foo/*"})
	if err != nil {
		t.Error(err.Error())
	}

	err = DecryptTar(inFile, outFile, key)
	if err != nil {
		t.Error(err.Error())
	}

	fi, _ := os.Stat(outFile)
	// get the size
	size := fi.Size()
	if size < 187-4 || size > 187+4 {
		t.Errorf("%d %d", size, 187)
	}

	os.RemoveAll(inFile)
	os.RemoveAll(outFile)
}
