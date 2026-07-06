#!/bin/bash
set -e
set -x


for i in {1..50};do
echo "
dn: cn=group$i,ou=Groups,dc=example,dc=org
objectClass: groupOfNames
cn: group$i
member:
" > group.lidf
ldapadd -x -D "cn=admin,dc=example,dc=org" -w 'admin' -f group.lidf
done

