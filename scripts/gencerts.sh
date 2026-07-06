#!/usr/bin/env bash

SOURCE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${SOURCE_ROOT}/scripts/lib/init.sh"

# OUT_DIR can come in from the Makefile, so honor it.
# 即可以通过make OUT_DIR=xxx 的方式设置OUT_DIR的值,而不是用默认值
readonly LOCAL_OUTPUT_ROOT="${SOURCE_ROOT}/${OUT_DIR:-_output}"
readonly LOCAL_OUTPUT_CAPATH="${LOCAL_OUTPUT_ROOT}/cert"

readonly CERT_HOSTNAME="${CERT_HOSTNAME:-example-server.exzycloud.com},127.0.0.1,localhost"


function generate_server_certificate()
{
  local cert_dir=${1}
  if [ $# -ne 4 ];then
    lib::log::error "Usage: generate_server_certificate ./_output/certs example-server /CN=example-server 127.0.0.1,localhost"
    exit 1
  fi
  common_generate_certificate ${1} ${2} ${3} ${4} "server"
}

function generate_client_certificate()
{
  local cert_dir=${1}
  if [ $# -ne 4 ];then
    lib::log::error "Usage: generate_client_certificate ./_output/certs example-client /CN=example-client example-client"
    exit 1
  fi
  common_generate_certificate ${1} ${2} ${3} ${4} "client"
}

# Args:
#   $1 (the directory that certificate files to save)
#   $2 (the prefix of the certificate filename)
#   $3 (cert subject alternative name)
#   $4 (cert subject common name )
#   $5 (server mode: server or client)
function common_generate_certificate()
{
 local cert_dir=${1}
 local prefix=${2}
 local cert_cn=${3}  # 证书主题通用名称(Common Name)
 local cert_san=${4} # 证书主题备用名称(Subject Alternative Name)
 local mode=${5}

 if [ $# -ne 5 ];then
    lib::log::error "Usage: common_generate_certificate ./_output/certs example-server 127.0.0.1,localhost /CN=example-server server "
    exit 1
 fi

 # 证书主题通用名或者证书主题备用名至少设置一个
 if [ -z ${cert_san} -a -z ${cert_cn} ];then
       lib::log::error "cert common name \"${cert_cn}\"or subject alternative name \"${cert_san}\" at least set one"
       exit 1
 fi

 usage="serverAuth"
 if [ ${mode} = "client" ];then
  usage="clientAuth"
 fi

 mkdir -p "${cert_dir}"

 # 确认openssl是否安装
 lib::util::test_openssl_installed
 # 将当前路径入栈,并跳转到证书目录
 pushd "${cert_dir}"

 # 如果ca证书不存在, 则生成自签名ca证书
 if [ ! -r "ca.crt" ]; then
   lib::log::info "ca.crt not exist, trying to generate ca.art in ${cert_dir} "
   ${OPENSSL_BIN} genrsa -out ca.key 4096
   ${OPENSSL_BIN} req -x509 -new -nodes -sha512 -days 3650 \
      -subj "$cert_cn" \
      -key ca.key \
      -out ca.crt
 fi

 lib::log::info "Generate "${prefix}" certificates in ${cert_dir}"

 # 生成私钥
 ${OPENSSL_BIN} genrsa -out ${prefix}.key 4096
 # 生成证书签名请求
 ${OPENSSL_BIN} req -sha512 -new \
     -subj "$cert_cn" \
     -key ${prefix}.key \
     -out ${prefix}.csr

  v3ExtFILE=${prefix}_v3.ext


cat > ${v3ExtFILE} <<-EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = ${usage}
EOF

if [ ${cert_san} != "\"\"" ];then
  cat >> ${v3ExtFILE} <<-EOF
subjectAltName = @alt_names

[alt_names]
EOF

 # 按,切割证书可选主题
IFS=',' read -ra elements <<< "${cert_san}"

# 使用循环遍历主题，生成证书主题
 j=0
 for (( i=0; i<${#elements[@]}; i++ )); do
  element="${elements[$i]}"
  if [[ $element =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    # 如果是IP地址，则给 IP.* 赋值
    echo "IP.$((j=j+1)) = $element" >> ${v3ExtFILE}
  fi
 done

 j=0
 for (( i=0; i<${#elements[@]}; i++ )); do
  element="${elements[$i]}"
  if [[ ! $element =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    # 否则，给 DNS.* 赋值
    echo "DNS.$((j=j+1)) = $element" >> ${v3ExtFILE}
  fi
 done
fi


 ${OPENSSL_BIN} x509 -req -sha512 -days 3650 \
     -extfile ${v3ExtFILE}  \
     -CA ca.crt -CAkey ca.key -CAcreateserial \
     -in ${prefix}.csr \
     -out ${prefix}.crt

 # 跳回到上一次入栈的路径
 popd
}


# 加上`$*`这句后, 就允许外部通过<脚本名> <函数名> 来直接调用脚本内的函数
# 它会将所有位置参数合并为一个字符串，而不会像数组一样保留参数之间的分隔符。这在某些情况下可能会导致意外的结果，特别是如果参数包含空格或其他特殊字符。
# 即外部调用`scripts/gencerts.sh generate_client_certificate arg1 ""`时,`$*`等于`generate_client_certificate arg1 ""`
# 当由于`$*`的特性会把参数变为字符串,因此实际值为`generate_client_certificate arg1`, 导致""参数不会传递给generate_client_certificate
#$*

# 定义一个空数组
args_array=()
# 遍历输入的参数并添加到数组中
for arg in "$@"; do
    args_array+=("$arg")
done

func_args=""

# 取出第一个参数作为命令
command=${args_array[0]}
# 提取剩余参数列表, 并处理有可能的""入参
for (( i=1; i<${#args_array[@]}; i++ )); do
  element="${args_array[$i]}"
  if [ -z ${element} ];then
     func_args="${func_args} \"\""
  else
     func_args="${func_args} ${element}"
  fi
done

${command} ${func_args}
