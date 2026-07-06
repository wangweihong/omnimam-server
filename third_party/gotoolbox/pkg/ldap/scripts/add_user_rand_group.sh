#!/bin/bash
set -e
set -x

groupNum=50

for i in {1..6000};do
  j=$(((RANDOM % groupNum)+1))

rm modify.lidf || true
echo "dn: cn=group$j,ou=Groups,dc=example,dc=org
changetype: modify
add: member
member: cn=user$i,ou=People,dc=example,dc=org
" > modify.lidf

ldapmodify -x -D "cn=admin,dc=example,dc=org" -w 'admin' -f ./modify.lidf

done
