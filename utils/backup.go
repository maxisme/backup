package utils

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type ServerName = string

type RcloneConfig struct {
	Source      string `json:"src"`
	Destination string `json:"dest"`
}

type Server struct {
	Host             string   `json:"host"`           // destination ip/hostname of server
	Port             int      `json:"ssh-port"`       // which ssh port to use [default: 22]
	PersistDirectory bool     `json:"persist"`        // whether to keep the backed up directory after finished which will speed up future backups dramatically but take up space [default: true]
	ExcludeMounts    bool     `json:"exclude-mounts"` // whether to exclude mounts [default: false]
	RcloneDest       string   `json:"rclone-dest"`    // dest:path of rclone config
	ExcludeDirs      []string `json:"exclude-dirs"`   // directories on server to exclude from backup
	RootDir          string   `json:"root-dir"`       // root directory to backup - useful when you are mounting a fs to backup
}

type ServerEntry struct{ Server }

func (s *ServerEntry) UnmarshalJSON(b []byte) error {
	s.Server = Server{Port: 22, PersistDirectory: true, RootDir: "/"} // default values
	return json.Unmarshal(b, &s.Server)
}

type ServersConfig struct {
	Servers     map[ServerName]ServerEntry `json:"servers"`
	ExcludeDirs []string                   `json:"exclude-dirs"`   // directories on ALL servers to not backup
	Key         string                     `json:"encryption-key"` // key to encrypt the compressed server with
	PostCmd     string                     `json:"post-cmd"`       // command to be executed after backup
}

const (
	BackupDir          = "/backup/"
	TmpDir             = "/tmp/"
	EncryptionFileType = ".maxcrypt"
)

// BackupServers will backup the servers from the file in the config
func BackupServers(servers ServersConfig) {
	var wg sync.WaitGroup
	wg.Add(len(servers.Servers))
	for name, serverEntry := range servers.Servers {
		go func(servers ServersConfig, serverName string, server Server) {
			defer wg.Done()
			log.Printf("%s: Started backup\n", serverName)

			start := time.Now()
			// create directory to backup servers to
			backupDir := fmt.Sprintf("%s%s/", BackupDir, serverName)
			err := os.MkdirAll(backupDir, os.ModePerm)
			if err != nil {
				log.Printf("%s: %s\n", serverName, err.Error())
				return
			}

			// rsync contents of Server to directory
			args := getRsyncCmds(server, servers.ExcludeDirs, backupDir)
			log.Printf("%s: running: rsync %v\n", serverName, args)
			cmd := exec.Command("rsync", args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err = cmd.Run()
			if err != nil {
				log.Printf("%s: %s %s\n", serverName, fmt.Sprint(err), stderr.String())
			}

			// encrypt and compress directory
			compressedDirPath := TmpDir + serverName + "_" + strconv.FormatInt(time.Now().Unix(), 10) + EncryptionFileType
			if err := EncryptCompressDir(backupDir, compressedDirPath, servers.Key); err != nil {
				log.Printf("%s: Error encrypting/compressing: %s\n", serverName, err)
				return
			}

			if server.RcloneDest != "" {
				c := exec.Command("rclone", "copy", compressedDirPath, server.RcloneDest)
				log.Printf("%s: Running: %s\n", serverName, c.String())
				out, err := c.CombinedOutput()
				log.Printf("%s: rclone out: %v %v\n", serverName, out, err)
				_ = os.Remove(compressedDirPath)
			}

			if !server.PersistDirectory {
				if err := os.RemoveAll(backupDir); err != nil {
					log.Printf("%s: removeAll %s\n", serverName, err)
					return
				}
			}

			log.Printf("%s: Backed up in %s seconds\n", serverName, time.Since(start))
		}(servers, name, serverEntry.Server)
	}
	wg.Wait()

	if servers.PostCmd != "" {
		//  run command
		c := exec.Command("/bin/sh", "-c", servers.PostCmd)
		log.Println("Running: " + c.String())
		out, err := c.CombinedOutput()
		log.Printf("Ran final command '%s'\n%v\n%v\n", servers.PostCmd, string(out), err)
	}
}

func getRsyncCmds(server Server, excludeDirs []string, backupDir string) []string {
	args := []string{"-aAX", "--numeric-ids", "--delete"}

	if server.ExcludeMounts {
		args = append(args, "-x")
	}

	for _, dir := range server.ExcludeDirs {
		// server excludes
		args = append(args, fmt.Sprintf("--exclude=%s", dir))
	}
	for _, dir := range excludeDirs {
		// global excludeDirs
		args = append(args, fmt.Sprintf("--exclude=%s", dir))
	}

	// destination rsync cmds
	if server.Host == "" {
		if server.RootDir == "/" {
			panic("You can't backup the docker container... Customise the root-dir of server if mounting")
		}
		args = append(args, server.RootDir, backupDir)
	} else {
		args = append(args, "-e", fmt.Sprintf("ssh -p %d", server.Port))
		args = append(args, fmt.Sprintf("%s:%s", server.Host, server.RootDir), backupDir)
	}

	return args
}

func TempFileName() string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return filepath.Join(os.TempDir(), hex.EncodeToString(randBytes))
}

func EncryptCompressDir(dir, out, key string) error {
	f, err := os.Create(TempFileName() + ".tar.gz")
	if err != nil {
		return err
	}

	defer func() {
		log.Println(f.Close())
		log.Println(os.Remove(f.Name()))
	}()

	log.Printf("Taring %s\n", dir)
	w := bufio.NewWriter(f)
	if err := Tar(dir, w); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}

	log.Printf("Encrypting %s\n", dir)
	outFile, err := os.Create(out)
	if err != nil {
		return err
	}
	defer outFile.Close()
	w = bufio.NewWriter(outFile)
	f2, err := os.Open(f.Name())
	if err != nil {
		return err
	}
	defer f2.Close()
	r := bufio.NewReader(f2)
	if err := Encrypt(r, w, key); err != nil {
		return err
	}

	return w.Flush()
}

func DecryptTar(path, out, key string) error {
	inFile, err := os.Open(path)
	if err != nil {
		return err
	}
	outFile, err := os.Create(out)
	if err != nil {
		return err
	}
	err = Decrypt(inFile, outFile, key)
	return err
}
