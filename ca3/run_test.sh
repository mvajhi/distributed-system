#!/bin/bash

echo "Cleaning up old processes on ports 8001-8003..."
kill -9 $(lsof -t -i:8001,8002,8003) 2>/dev/null
sleep 1

mkdir -p results

echo "Compiling the server for faster execution..."
go build -o replica_server replica/main.go

echo "Starting replicas in background..."
# اجرای رپلیکای اول با اعمال تاخیر شبکه ۵۰۰ میلی‌ثانیه‌ای در پس‌زمینه
NETWORK_DELAY_MS=500 ./replica_server configs/replica1.json > /dev/null 2>&1 &
PID1=$!
./replica_server configs/replica2.json > /dev/null 2>&1 &
PID2=$!
./replica_server configs/replica3.json > /dev/null 2>&1 &
PID3=$!

sleep 3

echo "Running Scenario 1: Temporary Inconsistency..." > results/scenario1.txt
echo "1. Writing x=100 to Replica 1 (Eventual)" >> results/scenario1.txt
curl -s -X POST http://localhost:8001/put -d '{"key":"x", "value":"100", "consistency":"eventual"}' >> results/scenario1.txt

echo -e "\n\n2. Immediate GET from Replica 2 (Should fail as Key not found):" >> results/scenario1.txt
curl -s http://localhost:8002/get/x >> results/scenario1.txt

echo -e "\n\n3. Waiting 2 seconds for eventual sync..." >> results/scenario1.txt
sleep 2

echo -e "\n4. GET from Replica 2 after delay (Should have the data):" >> results/scenario1.txt
curl -s http://localhost:8002/get/x >> results/scenario1.txt


echo "Running Scenario 2: Replica Failure (Eventual Mode)..." > results/scenario2.txt
echo "1. Stopping Replica 3 (PID: $PID3)..." >> results/scenario2.txt
kill -9 $PID3
sleep 1

echo -e "\n2. Writing y=200 to Replica 1 (Eventual)" >> results/scenario2.txt
curl -s -X POST http://localhost:8001/put -d '{"key":"y", "value":"200", "consistency":"eventual"}' >> results/scenario2.txt

echo -e "\n\n3. Waiting 2 seconds for replication..." >> results/scenario2.txt
sleep 2

echo -e "\n4. GET from Replica 2 (Should have the data despite Replica 3 being down):" >> results/scenario2.txt
curl -s http://localhost:8002/get/y >> results/scenario2.txt

echo "Cleaning up remaining replicas..."
kill -9 $PID1 $PID2 2>/dev/null

echo "Tests completed. Check results/scenario1.txt and results/scenario2.txt"
