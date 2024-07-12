## Build Nethermind Via Pre-Built Image:
ARG baseimage=nethermindeth/nethermind
ARG tag=master
FROM $baseimage:$tag

RUN apt-get update && apt-get install -y jq && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Create version.txt
RUN dotnet /nethermind/nethermind.dll --version > /raw_version.txt && tail -n 1 /raw_version.txt > /version.txt

# Add genesis mapper script, startup script, and enode URL retriever script
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq
ADD mkconfig.jq /mkconfig.jq
ADD nethermind.sh /nethermind.sh
ADD enode.sh /hive-bin/enode.sh
ADD keystore-password /networkdata/keystore_password_filename
ADD miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json /networkdata/miner_keystores/miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json
ADD test.cfg /configs/test.cfg

# Set execute permissions for scripts
RUN chmod +x /nethermind.sh /hive-bin/enode.sh

# Expose networking ports
EXPOSE 8545 8551 30303 30303/udp

ENV NETHERMIND_HIVE_ENABLED true

ENTRYPOINT ["/nethermind.sh"]
