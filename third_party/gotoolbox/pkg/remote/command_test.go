package remote_test

import (
	"bufio"
	"fmt"
	"io"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/remote"
)

const (
	host       = "192.168.134.197"
	user       = "vagrant"
	password   = "vagrant"
	privateKey = "./testdata/id_rsa"
	knownHosts = "./testdata/known_hosts"
)

func TestPasswordExec(t *testing.T) {
	Convey("remote command with password", t, func() {
		Convey("read file", func() {
			cmd, err := remote.NewSSHBuilder().WithEndpoint(host).WithUser(user).AddAuthFromPassword(password).BuildCommand()
			So(err, ShouldBeNil)
			_, err = cmd.Output("cat /etc/hosts")
			So(err, ShouldBeNil)
		})
	})
}

func TestPrivateKeyExec(t *testing.T) {
	Convey("remote command with password", t, func() {
		Convey("read file", func() {
			cmd, err := remote.NewSSHBuilder().WithEndpoint(host).WithUser(user).AddAuthFromPrivateKeyFile(privateKey, "").BuildCommand()
			So(err, ShouldBeNil)
			_, err = cmd.Output("cat /etc/hosts")
			So(err, ShouldBeNil)
		})
	})
}

func TestNewSSHBuilder(t *testing.T) {
	Convey("从输入读取命令，并不停的读取输出", t, func() {
		session, err := remote.NewSSHBuilder().WithEndpoint(host).WithUser(user).AddAuthFromPrivateKeyFile(privateKey, "").BuildSession()
		So(err, ShouldBeNil)

		_, err = session.Exec("ls -sl")
		So(err, ShouldBeNil)
		/*
		   Welcome to Ubuntu 20.04.6 LTS (GNU/Linux 5.4.0-166-generic x86_64)

		   .]0;vagrant@vagrant: ~vagrant@vagrant:~$ total 23328

		     564 -rw-r--r-- 1 root root   576282 May 31 15:54 bk-ci-charts.tgz

		       4 -rwxr-xr-x 1 root root     1241 Jun  7 15:48 containerd_proxy.sh

		       4 drwxr-xr-x 2 root root     4096 Jun 11 11:03 example

		       4 -rw-r--r-- 1 root root      584 Jun 12 15:45 id_pub

		       4 -rwxr-xr-x 1 root root      469 Jun  7 16:08 install_go.sh

		       4 -rwxr-xr-x 1 root root      168 May 27 15:10 install_packer.sh

		   22484 -rw-r--r-- 1 root root 23021917 Aug 19  2023 packer_1.9.4_linux_amd64.zip

		     112 -rw-r--r-- 1 root root   111319 Jun  4 11:26 pipeline.yaml

		     132 -rw-r--r-- 1 root root   132431 Jun 11 10:50 release.yaml

		      12 -rw-r--r-- 1 root root     9194 Jun  4 14:47 tekton-dashboard-release.yaml

		       4 drwxr-xr-x 2 root root     4096 May 27 15:14 template
		*/

		// 读取终端输出
		reader := bufio.NewReader(session.StdoutPipe)
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
					break
				}
				fmt.Println(line)
			}
		}()

		// 输入命令
		_, err = fmt.Fprintln(session.StdinPipe, "tail -f /var/log/dmesg")
		So(err, ShouldBeNil)

		time.Sleep(20 * time.Second)

	})
}
