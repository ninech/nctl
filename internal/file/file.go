package file

import (
	"bufio"
	"os"

	storage "github.com/ninech/apis/storage/v1alpha1"
)

// ReadSSHKeys reads SSH keys from the file specified by path
func ReadSSHKeys(path string) ([]storage.SSHKey, error) {
	if path == "" {
		return []storage.SSHKey{}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileScanner := bufio.NewScanner(file)
	fileScanner.Split(bufio.ScanLines)

	sshkeys := []storage.SSHKey{}
	for fileScanner.Scan() {
		sshkeys = append(sshkeys, storage.SSHKey(fileScanner.Text()))
	}

	return sshkeys, nil
}
