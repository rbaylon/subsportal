#!/bin/ksh

daemon=$1
rundir=$2
while true 
do 
   pcount=`ps aux | grep $daemon | wc -l`
   if [[ $pcount -lt 3 ]]; then
        echo "Captive portal not running. Restarting"
        cd $rundir
        nohup ./$daemon &

   else 
        echo "Captive portal running. Nothing to do here."
   fi
   sleep 60 
done
