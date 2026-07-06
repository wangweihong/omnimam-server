#!/bin/bash
set -e
set -x

for i in {1..6000};do
echo "
dn: cn=user$i,ou=People,dc=example,dc=org
objectClass: person
objectClass: inetOrgPerson
sn: doe
cn: user$i
mail: user$i@example.com
userpassword: user$i

" > users.lidf
ldapadd -x -D "cn=admin,dc=example,dc=org" -w 'admin' -f users.lidf
done

