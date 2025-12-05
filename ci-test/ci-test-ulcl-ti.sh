#!/bin/bash

##########################
#
# usage:
# ./ci-test-ulcl-ti.sh <test-name>
#
# e.g. ./ci-test-ulcl-ti.sh TestULCLTrafficInfluence
#
##########################

TEST_POOL="TestULCLTrafficInfluence"

# check if the test name is in the allowed test pool
if [[ ! "$1" =~ ^($TEST_POOL)$ ]]; then
    echo "Error: test name '$1' is not in the allowed test pool"
    echo "Allowed tests: $TEST_POOL"
    exit 1
fi

# run test
echo "Running test... $1"

LOG_FILE="$1.log"
docker exec ci /bin/bash -c "cd test && ./test-ulcl-ti.sh $1" 2>&1 | tee "$LOG_FILE"
exit_code=${PIPESTATUS[0]}

echo "Test completed with exit code: $exit_code"
if [ $exit_code -ne 0 ]; then
    echo "Test failed. Logs saved to: $LOG_FILE"
else
    echo "Test passed. Logs saved to: $LOG_FILE"
fi
exit $exit_code