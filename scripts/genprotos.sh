#!/usr/bin/env bash

SOURCE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${SOURCE_ROOT}/scripts/lib/init.sh"

readonly  exclude_files=()
# Args:
#   $1 (the directory that proto files generate )
function generate_protos()
{
 local proto_dirs=${1}

 pushd ${proto_dirs}
 # remove old pb file
 find ./ -name "*.pb.go" |xargs rm -rf

 # 生成新的pb.go文件
 for pbfile in `find  -name "*.proto"` ; do
   echo $pbfile
   # pb.go文件生成规则.
   # https://protobuf.dev/reference/go/go-generated/
   # 方式一:    protoc --proto_path=. --go_out=../apis --go_opt=paths=source_relative  $pbfile
   #  --proto_path 用于指定proto文件所在的路径。假如$pbfile为version.proto,实际去查找的路径为--proto_path/version.proto
   #  --go_out 用于指定pb.go文件输出的根路径.
   #      `./`表示当前执行命令的路径(非相对于.proto文件所在的路径)，
   #      `../`表示当前执行命令的路径的上一级路径(非相对于.proto文件所在的路径)，
   #  --go_opt=paths=source_relative,将会读取--proto_path路径下(当前路径,即${proto_dirs})的proto文件(version.proto或者version/version.proto)
   #  并输出version.pb.go文件到--go_out指定的路径(../apis)。
   #      如果pb文件为version,则输出路径为../apis/version.pb.go
   #      如果pb文件为version/version.pb,则输出路径为../apis/version/version.pb.go
   #      这种方式生成文件的路径不会被pb文件中的`option go_package`的影响。
   # 方式二：  protoc --proto_path=. --go_out=./ --go_opt=module=github.com/wangweihong/omnimam/backend/internal/pkg/grpcserver/apis  $pbfile
   #      module=$(Prefix), 其中${Prefix)为模块路径前缀。
   #      采用这种方式, 则要求proto文件中`option go_package`必须包含${Prefix}, 否则会报错。
   #      生成的pb文件路径为--go_out指定的路径/(`option go_package`指定的报名 - ${Prefix}
   #      如--go_out=./, proto的`option go_path="example.com/project/proto/version`,${Prefix}为"example.com/project/proto
   #      则输出路径为./proto/version/*.pb.go.
   # 方式三：  protoc --proto_path=. --go_out=../ --go_opt=paths=import  $pbfile
   #      pb.go文件的输出路径为--go_out指定的路径加上proto文件中指定的option go_package路径的影响
   #      如version.proto文件为option go_package = "github.com/wangweihong/omnimam/backend/internal/pkg/grpcserver/apis/version";
   #      则输出路径为../github.com/wangweihong/omnimam/backend/internal/pkg/grpcserver/apis/version
   #      *注意*: 虽然也可以设置`option go_package="apis/version", 但如果protos之间存在import,生成的pb.go文件中import (apis/version)
   #             导致go编译因为找不到apis/version而出错。

   # 这里采用第一种方式,
   # 必须设置plugins=grpc, 否则将不会生成服务注册函数
   protoc --proto_path=. --go_out=plugins=grpc:../apis --go_opt=paths=source_relative  $pbfile
 done

 popd
}

# 加上这句后, 就允许外部通过<脚本名> <函数名> 来直接调用脚本内的函数
$*