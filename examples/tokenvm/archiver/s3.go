package archiver

import "encoding/json"

var _ Archiver = (*S3Archiver)(nil)

type S3Archiver struct {
}

func CreateS3Archiver(configBytes []byte) (*S3Archiver, error) {
	var archiver S3Archiver
	err := json.Unmarshal(configBytes, &archiver)
	if err != nil {
		return nil, err
	}

	return &archiver, nil
}

func (s3a *S3Archiver) Put(k []byte, v []byte) error {
	return nil
}

func (s3a *S3Archiver) Exists(k []byte) (bool, error) {
	return false, nil
}

func (s3a *S3Archiver) Get(k []byte) ([]byte, error) {
	return nil, nil
}
