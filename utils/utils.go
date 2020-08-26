package utils

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// https://github.com/Xeoncross/go-aesctr-with-hmac/blob/master/crypt.go
const (
	KeyLen     int  = 32
	BufferSize int  = 16 * 1024
	IvSize     int  = 16
	V1         byte = 0x1
	hmacSize        = sha512.Size
)

// ErrInvalidHMAC for authentication failure
var ErrInvalidHMAC = errors.New("invalid HMAC")

// Encrypt the stream using the given AES-CTR and SHA512-HMAC key
func Encrypt(in io.Reader, out io.Writer, key string) (err error) {

	iv := make([]byte, IvSize)
	_, err = rand.Read(iv)
	if err != nil {
		return err
	}

	k, err := hex.DecodeString(key)
	if err != nil {
		return err
	}

	AES, err := aes.NewCipher(k)
	if err != nil {
		return err
	}

	ctr := cipher.NewCTR(AES, iv)
	HMAC := hmac.New(sha512.New, k) // https://golang.org/pkg/crypto/hmac/#New

	// Version
	_, err = out.Write([]byte{V1})
	if err != nil {
		return
	}

	w := io.MultiWriter(out, HMAC)

	_, err = w.Write(iv)
	if err != nil {
		return
	}

	buf := make([]byte, BufferSize)
	for {
		n, err := in.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n != 0 {
			outBuf := make([]byte, n)
			ctr.XORKeyStream(outBuf, buf[:n])
			_, err = w.Write(outBuf)
			if err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
	}

	_, err = out.Write(HMAC.Sum(nil))

	return err
}

// Decrypt the stream and verify HMAC using the given AES-CTR and SHA512-HMAC key
// Do not trust the out io.Writer contents until the function returns the result
// of validating the ending HMAC hash.
func Decrypt(in io.Reader, out io.Writer, key string) (err error) {

	// Read version (up to 0-255)
	var version int8
	err = binary.Read(in, binary.LittleEndian, &version)
	if err != nil {
		return
	}

	iv := make([]byte, IvSize)
	_, err = io.ReadFull(in, iv)
	if err != nil {
		return
	}

	k, err := hex.DecodeString(key)
	if err != nil {
		return err
	}

	AES, err := aes.NewCipher(k)
	if err != nil {
		return
	}

	ctr := cipher.NewCTR(AES, iv)
	h := hmac.New(sha512.New, k)
	h.Write(iv)
	mac := make([]byte, hmacSize)

	w := out

	buf := bufio.NewReaderSize(in, BufferSize)
	var limit int
	var b []byte
	for {
		b, err = buf.Peek(BufferSize)
		if err != nil && err != io.EOF {
			return
		}

		limit = len(b) - hmacSize

		// We reached the end
		if err == io.EOF {

			left := buf.Buffered()
			if left < hmacSize {
				return errors.New("not enough left")
			}

			copy(mac, b[left-hmacSize:left])

			if left == hmacSize {
				break
			}
		}

		h.Write(b[:limit])

		// We always leave at least hmacSize bytes left in the buffer
		// That way, our next Peek() might be EOF, but we will still have enough
		outBuf := make([]byte, int64(limit))
		_, err = buf.Read(b[:limit])
		if err != nil {
			return
		}
		ctr.XORKeyStream(outBuf, b[:limit])
		_, err = w.Write(outBuf)
		if err != nil {
			return
		}

		if err == io.EOF {
			break
		}
	}

	if !hmac.Equal(mac, h.Sum(nil)) {
		return ErrInvalidHMAC
	}

	return nil
}

func Tar(src string, writer io.Writer) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("unable to tar files - %v", err.Error())
	}

	gzw, _ := gzip.NewWriterLevel(writer, gzip.BestSpeed)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(strings.Replace(file, src, "", -1), string(filepath.Separator))

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		f.Close()

		return nil
	})
}

// FileToServers converts a file to a ServersConfig
func FileToServers(path string) (servers ServersConfig, err error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, &servers)
	if len(servers.Key) != KeyLen {
		err = fmt.Errorf("server key in config must be %d chars", KeyLen)
		return
	}
	return
}
