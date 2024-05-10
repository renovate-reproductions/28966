#!/bin/bash

# the backend requires the config file to be 0600 to work
chmod 0600 conf/config.json

echo -n "Start backend...."
BACKEND_LOG=`mktemp`
./backend -config conf/config.json 2> $BACKEND_LOG &
BACKEND_PID=$!
# wait for the backend to be ready
while [ "`grep 'Adding .* bridges' $BACKEND_LOG`" == "" ]
do
	sleep 0.1
done
echo " done"


echo -n "Start distributor...."
DISTRIBUTOR_LOG=`mktemp`
./distributors -name moat -config conf/config.json 2> $DISTRIBUTOR_LOG &
DISTRIBUTOR_PID=$!
# wait for the distributor to be ready
while [ "`grep 'Adding .* resources of type ' $DISTRIBUTOR_LOG`" == "" ]
do
	sleep 0.1
done
echo " done"

exit_() {
	kill $BACKEND_PID
	kill $DISTRIBUTOR_PID
	rm $BACKEND_LOG
	rm $DISTRIBUTOR_LOG
	exit $1
}


BRIDGE=`curl -s http://127.0.0.1:7500/moat/circumvention/defaults  | jq '.settings[1].bridges.bridge_strings[1]'`

if [[ "$BRIDGE" != \"obfs4* ]]
then
	echo ""
	echo "There was not a valid bridge:"
	echo $BRIDGE

	echo ""
	echo "Backend log:"
	cat $BACKEND_LOG
	echo ""
	echo "Distributor log:"
	cat $DISTRIBUTOR_LOG

	exit_ 1
fi

exit_ 0
