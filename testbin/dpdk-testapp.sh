#!/usr/bin/env bash

set -o errexit
set -o pipefail

function setup_dpdk_testapp {
    kubectl apply -f ../examples/example-dpdk-testapp.yaml
    kubectl wait -n power-manager --for=condition=available --timeout=180s deployment/dpdk-testapp

    POD=$(kubectl get pods -n power-manager -l app=dpdk-testapp \
        -ojsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
    NODE=$(kubectl get pods -n power-manager -l app=dpdk-testapp \
        -ojsonpath='{range .items[*]}{.spec.nodeName}{"\n"}{end}')
    until SERVER_CPUS=$(kubectl get powernode "$NODE" -n power-manager -o \
        jsonpath='{range .spec.powerContainers[?(@.name=="server")]}{.exclusiveCpus}{end}' |
        tr -d '[]' | tr ',' '\n' | awk 'BEGIN{n=0; ORS=","} {print n"@"$1; n++}' | sed 's/,$//'); do
        echo "PowerNode not yet updated. Retrying..."
        sleep 2
    done
    SERVER_CPUS_NUM=$(($(echo "$SERVER_CPUS" | grep -o '@' | wc -l) - 1))
    until CLIENT_CPUS=$(kubectl get powernode "$NODE" -n power-manager -o \
        jsonpath='{range .spec.powerContainers[?(@.name=="client")]}{.exclusiveCpus}{end}' |
        tr -d '[]' | tr ',' '\n' | awk 'BEGIN{n=0; ORS=","} {print n"@"$1; n++}' | sed 's/,$//'); do
        echo "PowerNode not yet updated. Retrying..."
        sleep 2
    done
    CLIENT_CPUS_NUM=$(($(echo "$CLIENT_CPUS" | grep -o '@' | wc -l) - 1))

    kubectl exec -n power-manager "$POD" -c server -- \
        tmux new-session -d "dpdk-testpmd --no-pci --lcores $SERVER_CPUS --huge-dir=\"/hugepages-1Gi\" \
        --vdev=\"net_memif0,role=server,socket=/var/run/memif/memif1.sock\" -- \
        --rxq=$SERVER_CPUS_NUM --txq=$SERVER_CPUS_NUM --nb-cores=$SERVER_CPUS_NUM \
        -i -a --rss-udp --forward-mode=csum --workload-scale=$WORKLOAD_SCALE"

    sleep 1

    kubectl exec -n power-manager "$POD" -c client -- \
        tmux new-session -d "dpdk-testpmd --lcores $CLIENT_CPUS --file-prefix=client --no-pci \
        --huge-dir=\"/hugepages-1Gi\" --vdev=\"net_memif0,role=client,socket=/var/run/memif/memif1.sock\" -- \
        --rxq=$SERVER_CPUS_NUM --txq=$SERVER_CPUS_NUM --nb-cores=$CLIENT_CPUS_NUM \
        -i -a --rss-udp --forward-mode=txonly"

    echo "Deployed successfully"
}

function delete_dpdk_testapp {
    kubectl delete -f ../examples/example-dpdk-testapp.yaml
}

# Helper for selecting workload scale:
#   AMD EPYC 9654 96-Core Processor, Min and Max Freq set to 50% (2055 MHz)
#   Client -> 7 CPUs, Server -> 31 CPUs, 4.42 server/client ratio
#   Workload Scale 1 = 10% mean busyness
#   Workload Scale 4 = 37% mean busyness
#   Workload Scale 8 = 64% mean busyness
#   Workload Scale 12 = 91% mean busyness

: "${KUBECONFIG:=$HOME/.kube/config}"

if [[ $1 == "-d" ]]; then
    delete_dpdk_testapp
    exit 0
fi

WORKLOAD_SCALE="$1"
setup_dpdk_testapp
exit 0
