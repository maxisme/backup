package utils

import (
	"os"
	"testing"
)

func TestEncryptCompressDir(t *testing.T) {
	inFile := "file.tar.gz.encr"
	outFile := "file.tar.gz"
	key := "90871670990532809087167099053280"
	err := EncryptCompressDir("./test", inFile, key)
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
