// Copyright 2018 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package handlers

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/mendersoftware/mender-artifact/artifact"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestHandlerRootfs(t *testing.T) {
	comp := artifact.NewCompressorGzip()

	// test if update type is reported correctly
	r := NewRootfsV1("", comp)
	assert.Equal(t, "rootfs-image", r.GetType())

	// test get update files
	r.update = &DataFile{Name: "update.ext4"}
	assert.Equal(t, "update.ext4", r.GetUpdateFiles()[0].Name)
	assert.Equal(t, 1, r.version)

	r = NewRootfsV2("", comp)
	assert.Equal(t, "rootfs-image", r.GetType())

	// test get update files
	r.update = &DataFile{Name: "update_next.ext4"}
	assert.Equal(t, "update_next.ext4", r.GetUpdateFiles()[0].Name)
	assert.Equal(t, 2, r.version)

	// test cppy
	n := r.Copy()
	assert.IsType(t, &Rootfs{}, n)
}

func TestRootfsCompose(t *testing.T) {
	comp := artifact.NewCompressorGzip()

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	f, err := ioutil.TempFile("", "update")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	r := NewRootfsV1(f.Name(), comp)
	err = r.ComposeHeader(tw, 1)
	assert.NoError(t, err)

	err = r.ComposeData(tw, 1)
	assert.NoError(t, err)

	// error compose data with missing data file
	r = NewRootfsV1("non-existing", comp)
	err = r.ComposeData(tw, 1)
	assert.Error(t, err)
	assert.Contains(t, errors.Cause(err).Error(),
		"no such file or directory")
}

func TestRootfsReadHeader(t *testing.T) {
	r := NewRootfsV1("custom", artifact.NewCompressorGzip())

	tc := []struct {
		data      string
		name      string
		shouldErr bool
		errMsg    string
	}{
		{data: "invalid", name: "headers/0000/files", shouldErr: true,
			errMsg: "error validating data"},
		{data: `{"files":["update.ext4", "next_update.ext4"]}`,
			name: "headers/0000/files", shouldErr: false},
		{data: `1212121212121212121212121212`,
			name: "headers/0000/checksums/update.ext4.sum", shouldErr: false},
		{data: "", name: "headers/0000/non-existing", shouldErr: true,
			errMsg: "unsupported file"},
		{data: "data", name: "headers/0000/type-info", shouldErr: false},
		{data: "", name: "headers/0000/meta-data", shouldErr: false},
		{data: "", name: "headers/0000/scripts/pre/my_script", shouldErr: false},
		{data: "", name: "headers/0000/scripts/post/my_script", shouldErr: false},
		{data: "", name: "headers/0000/scripts/check/my_script", shouldErr: false},
		{data: "", name: "headers/0000/signatures/update.sig", shouldErr: false},
	}

	for _, test := range tc {
		buf := bytes.NewBuffer(nil)

		tw := tar.NewWriter(buf)
		err := tw.WriteHeader(&tar.Header{
			Name: "not-needed",
			Size: int64(len(test.data)),
		})
		assert.NoError(t, err)
		_, err = tw.Write([]byte(test.data))
		assert.NoError(t, err)
		err = tw.Close()
		assert.NoError(t, err)

		tr := tar.NewReader(buf)
		_, err = tr.Next()
		assert.NoError(t, err)

		err = r.ReadHeader(buf, test.name)
		if test.shouldErr {
			assert.Error(t, err)
			if test.errMsg != "" {
				assert.Contains(t, errors.Cause(err).Error(), test.errMsg)
			}
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestRootfsReadData(t *testing.T) {
	r := NewRootfsInstaller()

	buf := bytes.NewBuffer([]byte("some data"))
	err := r.Install(buf, nil)
	assert.NoError(t, err)

	data := bytes.NewBuffer(nil)
	r.InstallHandler = func(r io.Reader, df *DataFile) error {
		_, err := io.Copy(data, r)
		return err
	}
	err = r.Install(buf, nil)
	assert.NoError(t, err)
	assert.Equal(t, "some data", data.String())

}
