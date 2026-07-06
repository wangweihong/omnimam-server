# 遍历pkg和internal/pkg两个目录, 如果doc.go文件不存在则创建
for top in pkg internal/pkg
do
    for d in $(find $top -type d)
    do
       if [ ! -f $d/doc.go ]; then
            if ls $d/*.go > /dev/null 2>&1; then
               echo $d/doc.go
               echo "package $(basename $d) // import \"github.com/wangweihong/omnimam/$d\"" > $d/doc.go
            fi
       fi
    done
done