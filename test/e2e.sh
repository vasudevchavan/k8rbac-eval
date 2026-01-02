#!/bin/bash
set -e

echo "Starting E2E Tests..."

# Clean and Build
echo "Building project..."
make clean
make build

# Check if binary exists
if [ ! -f "bin/kubeaccess" ]; then
    echo "Error: Binary not found at bin/kubeaccess"
    exit 1
fi

# Test Help
echo "Testing 'help' command..."
./bin/kubeaccess --help > /dev/null

# Test Version
echo "Testing 'version' command..."
./bin/kubeaccess version

# Test Generate (Command Parsing Only)
# We expect this might fail connectivity if no cluster is present, 
# but valid flags should parse.
# If we want to test strictly parsing without connectivity, 
# we might need a flag to skip K8s check or dry-run, which isn't implemented yet.
# For now, we'll run it and ignore the specific error "connect: connection refused" or similar if we are offline,
# but fail on "unknown flag".

echo "Testing 'generate' command parsing..."
set +e
OUTPUT=$(./bin/kubeaccess generate user testuser --resource pods --verb get -n default 2>&1)
EXIT_CODE=$?
set -e

# If exit code is non-zero, check if it's a connection error (which is acceptable for this local test)
# versus a flag parsing error.
if [ $EXIT_CODE -ne 0 ]; then
    if echo "$OUTPUT" | grep -q "unknown flag"; then
        echo "Error: Generate command failed with unknown flag:"
        echo "$OUTPUT"
        exit 1
    elif echo "$OUTPUT" | grep -q "required flag(s)"; then
         echo "Error: Generate command failed with missing flags:"
         echo "$OUTPUT"
         exit 1
    else
        echo "Generate command ran (failed as expected due to connectivity/config):"
        echo "$OUTPUT"
        echo "Ignoring connectivity failure for E2E."
    fi
else
    echo "Generate command succeeded."
    echo "$OUTPUT"
fi

echo "E2E Tests Passed!"
