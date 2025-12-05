#!/bin/bash

##########################
#
# usage:
# ./ci-test-ulcl-mp.sh <test-name>
#
# e.g. ./ci-test-ulcl-mp.sh <TestULCLMultiPathCi1 | TestULCLMultiPathCi2>
#
##########################

TEST_POOL="TestULCLMultiPathCi1|TestULCLMultiPathCi2"

# check if the test name is in the allowed test pool
if [[ ! "$1" =~ ^($TEST_POOL)$ ]]; then
    echo "Error: test name '$1' is not in the allowed test pool"
    echo "Allowed tests: $TEST_POOL"
    exit 1
fi

# run test
echo "Running test... $1"

LOG_FILE="test_logs_$1_$(date +%Y%m%d_%H%M%S).log"
echo "Logging to $LOG_FILE"

case "$1" in
    "TestULCLMultiPathCi1")
        docker exec ci-1 /bin/bash -c "cd test && ./test-ulcl-mp.sh $1" | tee "$LOG_FILE"
        exit_code=${PIPESTATUS[0]}
    ;;
    "TestULCLMultiPathCi2")
        docker exec ci-2 /bin/bash -c "cd test && ./test-ulcl-mp.sh $1" | tee "$LOG_FILE"
        exit_code=${PIPESTATUS[0]}
    ;;
esac

echo "Test completed with exit code: $exit_code"
exit $exit_code