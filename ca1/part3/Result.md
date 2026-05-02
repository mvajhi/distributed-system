» vagrant up                                                                                                                                   1 ↵
Bringing machine 'default' up with 'virtualbox' provider...
==> default: Importing base box 'gusztavvargadr/docker-linux'...
==> default: Matching MAC address for NAT networking...
==> default: Setting the name of the VM: DS_HW1_Docker_VM
==> default: Clearing any previously set network interfaces...
==> default: Preparing network interfaces based on configuration...
    default: Adapter 1: nat
    default: Adapter 2: hostonly
==> default: Forwarding ports...
    default: 22 (guest) => 2222 (host) (adapter 1)
==> default: Running 'pre-boot' VM customizations...
==> default: Booting VM...
==> default: Waiting for machine to boot. This may take a few minutes...
    default: SSH address: 127.0.0.1:2222
    default: SSH username: vagrant
    default: SSH auth method: private key
    default: 
    default: Vagrant insecure key detected. Vagrant will automatically replace
    default: this with a newly generated keypair for better security.
    default: 
    default: Inserting generated public key within guest...
    default: Removing insecure key from the guest if it's present...
    default: Key inserted! Disconnecting and reconnecting using new SSH key...
==> default: Machine booted and ready!
==> default: Checking for guest additions in VM...
==> default: Configuring and enabling network interfaces...
==> default: Mounting shared folders...
    default: /home/mvajhi/code/dist/ca1/part3 => /vagrant
==> default: Running provisioner: shell...
    default: Running: inline script
    default:  Image vagrant-calc-service Building
    default: #1 [internal] load local bake definitions
    default: #1 reading from stdin 492B done
    default: #1 DONE 0.0s
    default: 
    default: #2 [internal] load build definition from Dockerfile
    default: #2 transferring dockerfile: 232B done
    default: #2 DONE 0.0s
    default: 
    default: #3 [internal] load metadata for docker.io/library/golang:alpine
    default: #3 ...
    default: 
    default: #4 [internal] load metadata for docker.io/library/alpine:latest
    default: #4 DONE 0.5s
    default: 
    default: #3 [internal] load metadata for docker.io/library/golang:alpine
    default: #3 DONE 0.5s
    default: 
    default: #5 [internal] load .dockerignore
    default: #5 transferring context: 2B done
    default: #5 DONE 0.0s
    default: 
    default: #6 [internal] load build context
    default: #6 DONE 0.0s
    default: 
    default: #7 [stage-1 1/3] FROM docker.io/library/alpine:latest@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
    default: #7 resolve docker.io/library/alpine:latest@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 0.0s done
    default: #7 DONE 0.0s
    default: 
    default: #8 [builder 1/4] FROM docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
    default: #8 resolve docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
    default: #8 ...
    default: 
    default: #6 [internal] load build context
    default: #6 transferring context: 2.45kB done
    default: #6 DONE 0.1s
    default: 
    default: #8 [builder 1/4] FROM docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
    default: #8 resolve docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 0.0s done
    default: #8 sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1 32B / 32B 0.1s done
    default: #8 sha256:973f1b4898e9c2c22c54ff9d1bc2e2d45d59d4c2e53c60281dde84ef43df4c79 126B / 126B 0.1s done
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 0B / 67.22MB 0.2s
    default: #8 sha256:4c3aca1499f06ff02939c8f744463049feecb3965f1319b86b5e91790a9631e6 0B / 290.24kB 0.2s
    default: #8 sha256:4c3aca1499f06ff02939c8f744463049feecb3965f1319b86b5e91790a9631e6 290.24kB / 290.24kB 0.6s done
    default: #8 ...
    default: 
    default: #7 [stage-1 1/3] FROM docker.io/library/alpine:latest@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
    default: #7 sha256:6a0ac1617861a677b045b7ff88545213ec31c0ff08763195a70a4a5adda577bb 3.86MB / 3.86MB 5.2s done
    default: #7 extracting sha256:6a0ac1617861a677b045b7ff88545213ec31c0ff08763195a70a4a5adda577bb 0.2s done
    default: #7 DONE 5.5s
    default: 
    default: #8 [builder 1/4] FROM docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 2.10MB / 67.22MB 5.3s
    default: #8 ...
    default: 
    default: #9 [stage-1 2/3] WORKDIR /root/
    default: #9 DONE 0.1s
    default: 
    default: #8 [builder 1/4] FROM docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
    default: #8 extracting sha256:4c3aca1499f06ff02939c8f744463049feecb3965f1319b86b5e91790a9631e6
    default: #8 extracting sha256:4c3aca1499f06ff02939c8f744463049feecb3965f1319b86b5e91790a9631e6 0.2s done
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 6.29MB / 67.22MB 8.3s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 10.49MB / 67.22MB 11.9s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 14.68MB / 67.22MB 15.5s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 18.87MB / 67.22MB 19.2s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 23.07MB / 67.22MB 22.8s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 27.26MB / 67.22MB 26.6s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 31.46MB / 67.22MB 30.0s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 35.65MB / 67.22MB 33.8s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 39.85MB / 67.22MB 37.5s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 44.04MB / 67.22MB 41.0s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 48.23MB / 67.22MB 44.7s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 52.43MB / 67.22MB 48.2s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 56.62MB / 67.22MB 51.9s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 60.82MB / 67.22MB 55.5s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 65.01MB / 67.22MB 59.3s
    default: #8 sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 67.22MB / 67.22MB 61.1s done
    default: #8 extracting sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f
    default: #8 extracting sha256:b55da06e3b41084804b2e5dbba71d35d26478b19ba8055d07a393cae22e9935f 4.5s done
    default: #8 DONE 65.8s
    default: 
    default: #8 [builder 1/4] FROM docker.io/library/golang:alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
    default: #8 extracting sha256:973f1b4898e9c2c22c54ff9d1bc2e2d45d59d4c2e53c60281dde84ef43df4c79 0.0s done
    default: #8 extracting sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1 0.0s done
    default: #8 DONE 65.9s
    default: 
    default: #10 [builder 2/4] WORKDIR /app
    default: #10 DONE 1.0s
    default: 
    default: #11 [builder 3/4] COPY main.go .
    default: #11 DONE 0.1s
    default: 
    default: #12 [builder 4/4] RUN go build -o server main.go
    default: #12 DONE 40.3s
    default: 
    default: #13 [stage-1 3/3] COPY --from=builder /app/server .
    default: #13 DONE 0.1s
    default: 
    default: #14 exporting to image
    default: #14 exporting layers
    default: #14 exporting layers 0.7s done
    default: #14 exporting manifest sha256:7cb0e5c6551e1ca5fca1b2c672037bb4793cae1bdff588161541d3ecc68263e9
    default: #14 exporting manifest sha256:7cb0e5c6551e1ca5fca1b2c672037bb4793cae1bdff588161541d3ecc68263e9 0.0s done
    default: #14 exporting config sha256:3ed683d02592887a2f8576e4d7580f97bc8672742137fc6955d73d87ac06bd42 0.0s done
    default: #14 exporting attestation manifest sha256:1db78a29c8e0171abae62f34cf26ac72c3eaceddd219705e076884481fe452e9 0.0s done
    default: #14 exporting manifest list sha256:2d0b65c64aaa4d4c0c9110cab9c2bc03b43b0f7f394211b8bd922da61d0a46aa
    default: #14 exporting manifest list sha256:2d0b65c64aaa4d4c0c9110cab9c2bc03b43b0f7f394211b8bd922da61d0a46aa 0.0s done
    default: #14 naming to docker.io/library/vagrant-calc-service:latest 0.0s done
    default: #14 unpacking to docker.io/library/vagrant-calc-service:latest
    default: #14 unpacking to docker.io/library/vagrant-calc-service:latest 0.2s done
    default: #14 DONE 1.1s
    default: 
    default: #15 resolving provenance for metadata file
    default: #15 DONE 0.0s
    default:  Image vagrant-calc-service Built
    default:  Network vagrant_default Creating
    default:  Network vagrant_default Created
    default:  Container vagrant-calc-service-1 Creating
    default:  Container vagrant-calc-service-1 Created
    default:  Container vagrant-calc-service-1 Starting
    default:  Container vagrant-calc-service-1 Started

» curl "http://192.168.56.10:8080/health"           
OK

» curl "http://192.168.56.10:8080/compute?op=div&a=5&b=5"
{"operation":"div","a":5,"b":5,"result":1}
