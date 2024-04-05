package file

import (
	"bufio"
	"os"

	storage "github.com/ninech/apis/storage/v1alpha1"
)

func readSSHKeys(path string) ([]storage.SSHKey, error) {
	sshkeys := []storage.SSHKey{}

	if path == "" {
		return sshkeys, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return sshkeys, err
	}
	defer file.Close()

	fileScanner := bufio.NewScanner(file)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		sshkeys = append(sshkeys, storage.SSHKey(fileScanner.Text()))
	}

	return sshkeys, nil
}
