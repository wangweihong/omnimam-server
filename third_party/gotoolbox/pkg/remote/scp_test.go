package remote_test

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/remote"
)

func TestPrivateKeyUpload(t *testing.T) {
	Convey("remote upload with password", t, func() {
		Convey("read file", func() {
			cmd, err := remote.NewSSHBuilder().WithEndpoint(host).WithUser(user).AddAuthFromPrivateKeyFile(privateKey, "").BuildFile()
			So(err, ShouldBeNil)
			err = cmd.Upload("/home/vagrant/file", "./testdata/file")
			So(err, ShouldBeNil)
		})
	})
}

func TestPasswordList(t *testing.T) {
	Convey("remote list", t, func() {
		Convey("read file", func() {
			cmd, err := remote.NewSSHBuilder().WithEndpoint(host).WithUser(user).AddAuthFromPassword(password).BuildFile()
			So(err, ShouldBeNil)

			_, err = cmd.ListDirectory("/home/vagrant")
			So(err, ShouldBeNil)

			content, err := cmd.ReadFile("/home/vagrant/.bashrc")
			So(err, ShouldBeNil)

			fmt.Println(content)
		})
	})
}
