## Libri public testnet

The libri public test network has a small number of nodes running. We are actively seeking people
interested in helping test it, but please reach out to coordinate before sending any large load to 
it or spinning up your own Libri peers to join the network.

The current testnet seeds addresses are:

    LIBRI_TESTNET_SEEDS='35.229.82.151:30101 35.231.242.130:30102 35.229.82.151:30103'


### Simple file upload/download

The simplest way to run Libri Author (client) commands is from within a Libri Docker container. 
Start and enter a new container via

    docker run --rm -it --entrypoint /bin/bash daedalus2718/libri:snapshot
    
Once in, set the `LIBRI_TESTNET_SEEDS` via the expression above and then define some Libri 
environment variables so we don't have to pass them in as CLI args.

    export LIBRI_AUTHORLIBRARIANS=${LIBRI_TESTNET_SEEDS}
    export LIBRI_KEYCHAINSDIR='/tmp/libri/keys'
    export LIBRI_DATADIR='/tmp/libri/data'
    export LIBRI_PASSPHRASE='my super secret thingy'
    LOG_FILE=${LIBRI_DATADIR}/up.log
    
Confirm we can talk to the librarians.

    libri test health -a ${LIBRI_AUTHORLIBRARIANS}

Initialize the author keys and create the test file.

    libri author init
    
    mkdir -p ${LIBRI_DATADIR}
    echo 'Hello Libri!' > ${LIBRI_DATADIR}/test.up.txt
    
Upload the file and get the resulting envelope ID.

    libri author upload -f ${LIBRI_DATADIR}/test.up.txt 2>&1 | tee ${LOG_FILE}
    
    # grab envelope key from the log
    ENVELOPE_KEY=$(grep 'envelope_key' ${LOG_FILE} | sed -E 's/.*"envelope_key": "([^ "]*).*/\1/g') 
    echo "uploaded with envelope key '${ENVELOPE_KEY}'"
    
Download the file from the envelope ID and confirm it's the same as the uploaded file.

    libri author download -f ${LIBRI_DATADIR}/test.down.txt -e ${ENVELOPE_KEY} 
    
    cat ${LIBRI_DATADIR}/test.down.txt
    md5sum ${LIBRI_DATADIR}/test.*
    
When you're satisfied, exit the container.

    exit
    

### Spinning up a fleet of peers to join the Libri testnet.

Follow the GCP (cloud) directions on the [cloud deployment README] to spin up a stand-along cluster.
After running the `LIBRI_TESTNET_PEERS` command above, change the Librarian bootstrap addresses
to these testnet peers and re-apply Kubernetes config.

    sed -i -E "s/^(.*--bootstraps).*$/\1 '${LIBRI_TESTNET_SEEDS}'/g" libri.yml
    kubectl apply -f libri.yml