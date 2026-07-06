# 遍历pkg和internal/pkg两个目录, 清理所有的doc.go文件
for top in pkg internal/pkg
do
    for d in $(find $top -type d)
    do
        if [ -f $d/doc.go ];then
            rm $d/doc.go
        fi
    done
done
