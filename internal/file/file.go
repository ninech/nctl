package file

import (
	"bufio"
	"os"

	storage "github.com/ninech/apis/storage/v1alpha1"
)

func ReadSSHKeys(path string) ([]storage.SSHKey, error) {
	if path == "" {
		return nil, nil
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
