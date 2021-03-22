#!/bin/sh
rm -rf vendor bin logs
if(($# >= 1));then
   git add -A;git commit -m $1;git push
else
   git add -A;git commit -m 'update';git push
fi 
mkdir bin logs

go build -o bin/wjt_go .
go build -o bin/simple ssh/cmd1/simple.go
 



