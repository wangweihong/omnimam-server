#!/bin/bash

for i in {1..50};do
  ldapdelete -x -D "cn=admin,dc=example,dc=org" -w 'admin' cn=group$i,ou=Groups,dc=example,dc=org
done

for i in {1..6000};do
  ldapdelete -x -D "cn=admin,dc=example,dc=org" -w 'admin' cn=user$i,ou=People,dc=example,dc=org
done