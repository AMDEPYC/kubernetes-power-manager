# Kubernetes Power Manager

## Introduction

Utilizing a container orchestration engine like Kubernetes, CPU resources are allocated from a pool of platforms
entirely based on availability, without taking into account specific features of modern processors.

The Kubernetes Power Manager is a Kubernetes Operator that has been developed to provide cluster users with a mechanism
to dynamically request adjustment of worker node power management settings applied to cores allocated to the Pods. The
power management-related settings can be applied to individual cores or to groups of cores, and each
may have different policies applied. It is not required that every core in the system be explicitly managed by this
Kubernetes power manager. When the Power Manager is used to specify core power related policies, it overrides the
default settings

Powerful features of the AMD EPYC processors give users more precise control over CPU performance and power use on a
per-core basis. Yet, Kubernetes is purposefully built to operate as an abstraction layer between the workload and such
hardware capabilities as a workload orchestrator. Users of Kubernetes who are running performance-critical workloads
with particular requirements reliant on hardware capabilities encounter a challenge as a consequence.

The Kubernetes Power Manager bridges the gap between the container orchestration layer and hardware features enablement.

### Kubernetes Power Manager' main responsibilities:

- The Kubernetes Power Manager consists of two main components - the overarching manager which is deployed anywhere on a
  cluster and the power node agent which is deployed on each node you require power management capabilities.
- The overarching operator is responsible for the configuration and deployment of the power node agent, while the power
  node agent is responsible for the tuning of the cores as requested by the user.

### Use Cases:

-  Users may want to pre-schedule nodes to move to a performance profile during peak times to minimize spin up.
  At times not during peak, they may want to move to a power saving profile.
- *Unpredictable machine use.*
  Users may use machine learning through monitoring to determine profiles that predict a peak need for compute, to spin up
  ahead of time.
- *Power Optimization over Performance.*
  A user may be interested in fast response time, but not in maximal response time, so may choose to spin up cores on
  demand and only those cores used but want to remain in power-saving mode the rest of the time.

### Further Info:
  Please see the _diagrams-docs_ directory for diagrams with a visual breakdown of the power manager and its components.

## Functionality of the Kubernetes Power Manager

- **Power profiles**

  The user can arrange cores according to priority levels using this capability. When the system has extra power, it can
  be distributed among the cores according to their priority level. Although it cannot be guaranteed, the system will
  try to apply the additional power to the cores with the highest priority.
  There are four levels of priority available:
    1. Performance
    2. Balance Performance
    3. Balance Power
    4. Power

  The Priority level for a core is defined using its EPP (Energy Performance Preference) value, which is one of the
  options in the Power Profiles. If not all the power is utilized on the CPU, the CPU can put the higher priority cores
  up to Turbo Frequency (allows the cores to run faster).

- **Frequency Tuning**

  Frequency tuning allows the individual cores on the system to be sped up or slowed down by changing their frequency.
  This tuning is done via the Power Optimization Library.
  The min and max values for a core are defined in the Power Profile and the tuning is done after the core has been
  assigned by the Native CPU Manager.
  How exactly the frequency of the cores is changed is by simply writing the new frequency value to the
  /sys/devices/system/cpu/cpuN/cpufreq/scaling_max|min_freq file for the given core.

- **Dynamic CPU Frequency Management**

  CPU frequency can be dynamically adjusted based on CPU busyness. This can be achieved in two ways:

  1. **PowerProfile with schedutil governor**: This approach considers kernel-based CPU load for frequency scaling.

  2. **CPUScalingProfile**: This option leverages DPDK Telemetry to assess busyness, allowing for efficient
     CPU frequency management in poll-based applications, which are common in the networking domain.

  By utilizing these methods, it is possible to save energy even for workloads that may appear to fully
  utilize CPU resources from the kernel's perspective.

- **Time of Day**

  Time of Day is designed to allow the user to select a specific time of day that they can put all their unused CPUs
  into “sleep” state and then reverse the process and select another time to return to an “active” state.

-  **Scaling Drivers**
    * **P-State**
  
      AMD EPYC CPUs automatically employ the P_State CPU power scaling driver.
      The AMD P-State driver utilizes CPUFreq governors.
    * **acpi-cpufreq**
      
      The acpi-cpufreq driver setting operates much like the P-state driver but has a different set of available governors. For more information see [here](https://www.kernel.org/doc/html/v4.12/admin-guide/pm/cpufreq.html).
      One thing to note is that acpi-cpufreq reports the base clock as the frequency hardware limits however the P-state driver uses turbo frequency limits.
      Both drivers can make use of turbo frequency; however, acpi-cpufreq can exceed hardware frequency limits when using turbo frequency.
      This is important to take into account when setting frequencies for profiles.

- **Uncore**
  The largest part of modern CPUs is outside the actual cores. On AMD EPYC CPUs this is part is called the "Uncore" and has
  last level caches, PCI-Express, memory controller, QPI, power management and other functionalities.
  The previous deployment pattern was that an uncore setting was applied to sets of servers that are allocated as
  capacity for handling a particular type of workload. This is typically a one-time configuration today. The Kubenetes
  Power Manager now makes this dynamic and through a cloud native pattern. The implication is that the cluster-level
  capacity for the workload can then configured dynamically, as well as scaled dynamically.

- **Metric reporting**

  Processor-related metrics are available for scraping by Telegraf or similar metrics agent. Metrics are provided in Prometheus exposition format.
  Metrics are collected from various sources, like Linux PMU, MSR, or dedicated libraries.

## Prerequisites

* Node Feature Discovery ([NFD](https://github.com/kubernetes-sigs/node-feature-discovery)) should be deployed in the
  cluster before running the Kubernetes Power Manager. NFD is used to detect node-level features.
  Once detected, the user can instruct the Kubernetes Power Manager to deploy the Power Node Agent
  to Nodes with specific labels, allowing the Power Node Agent to take advantage of such
  features by configuring cores on the host to optimise performance for containerized workloads.
  Note: NFD is recommended, but not essential. Node labels can also be applied manually. See
  the [NFD repo](https://github.com/kubernetes-sigs/node-feature-discovery#feature-labels) for a full list of features
  labels.
* **Important**: In the kubelet configuration file the cpuManagerPolicy has to set to "static", and the reservedSystemCPUs must be set to
  the desired value:

````yaml
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
cgroupDriver: systemd
clusterDNS:
  - 10.96.0.10
clusterDomain: cluster.local
cpuManagerPolicy: "static"
cpuManagerReconcilePeriod: 0s
evictionPressureTransitionPeriod: 0s
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMinimumGCAge: 0s
kind: KubeletConfiguration
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
  verbosity: 0
memorySwap: { }
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
reservedSystemCPUs: "0"
resolvConf: /run/systemd/resolve/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 0s
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
````

## Deploying the Kubernetes Power Manager using Helm

The Kubernetes Power Manager includes a helm chart for the latest releases, allowing the user to easily deploy 
everything that is needed for the overarching operator and the node agent to run. The following versions are 
supported with helm charts:

* v2.0.0
* v2.1.0
* v2.2.0
* v2.3.0
* v2.3.1
* ocp-4.13-v2.3.1

When set up using the provided helm charts, the following will be deployed:

* The power-manager namespace
* The RBAC rules for the operator and node agent
* The operator deployment itself
* The operator's power config
* A shared power profile

To change any of the values the above are deployed with, edit the values.yaml file of the relevant helm chart.

To deploy the Kubernetes Power Manager using Helm, you must have Helm installed. For more information on installing 
Helm, see the installation guide here https://helm.sh/docs/intro/install/.

To install the latest version, use the following command:

`make helm-install`

To uninstall the latest version, use the following command:

`make helm-uninstall`

You can use the HELM_CHART and OCP parameters to deploy an older or Openshift specific version of the Kubernetes Power Manager:

`HELM_CHART=v1.1.0 OCP=true make helm-install`
`HELM_CHART=v1.1.0 make helm-install`
`HELM_CHART=v1.1.0 make helm-install`

Please note when installing older versions that certain features listed in this README may not be supported.

## Components

### Power Optimization Library

Power Optimization Library takes the desired configuration
for the cores associated with Exclusive Pods and tune them based on the requested Power Profile. The Power Optimization
Library will also facilitate the use of the C-States functionality.

### Power Node Agent

The Power Node Agent is also a containerized application deployed by the Kubernetes Power Manager in a DaemonSet. The
primary function of the node agent is to communicate with the node's Kubelet PodResources endpoint to discover the exact
cores that are allocated per container. The node agent watches for Pods that are created in your cluster and examines
them to determine which Power Profile they have requested and then sets off the chain of events that tunes the
frequencies of the cores designated to the Pod.

### Config Controller

The Kubernetes Power Manager will wait for the PowerConfig to be created by the user, in which the desired PowerProfiles
will be specified. The PowerConfig holds different values: what image is required, what Nodes the user wants to place
the node agent on and what PowerProfiles are required.

* powerNodeSelector: This is a key/value map used for defining a list of node labels that a node must satisfy in order
  for the Power Node Agent to be deployed.
* powerProfiles: The list of PowerProfiles that the user wants available on the nodes.

Once the Config Controller sees that the PowerConfig is created, it reads the values and then deploys the node agent on
to each of the Nodes that are specified. It then creates the PowerProfiles and extended resources. Extended resources
are resources created in the cluster that can be requested in the PodSpec. The Kubelet can then keep track of these
requests. It is important to use as it can specify how many cores on the system can be run at a higher frequency before
hitting the heat threshold.

Note: Only one PowerConfig can be present in a cluster. The Config Controller will ignore and delete and subsequent
PowerConfigs created after the first.

### Example

````yaml
apiVersion: "power.amdepyc.com/v1"
kind: PowerConfig
metadata:
  name: power-config
  namespace: power-manager
spec:
  powerNodeSelector:
    feature.node.kubernetes.io/power-node: "true"
  powerProfiles:
    - "performance"
    - "balance-performance"
    - "balance-power"
````

### Workload Controller

The Workload Controller is responsible for the actual tuning of the cores. The Workload Controller uses the Power
Optimization Library and requests that it creates the Pools. The Pools hold the PowerProfile associated with the cores
and the cores that need to be configured.

The PowerWorkload objects are created automatically by the PowerPod controller. This action is undertaken by the
Kubernetes Power Manager when a Pod is created with a container requesting exclusive cores and a PowerProfile.

PowerWorkload objects can also be created directly by the user via the PowerWorkload spec. This is only recommended when
creating the Shared PowerWorkload for a given Node, as this is the responsibility of the user. If no Shared
PowerWorkload is created, the cores that remain in the ‘shared pool’ on the Node will remain at their core frequency
values instead of being tuned to lower frequencies. PowerWorkloads are specific to a given node, so one is created for
each Node with a Pod requesting a PowerProfile, based on the PowerProfile requested.

### Example - automatically created PowerWorkload

````
apiVersion: "power.amdepyc.com/v1"
kind: PowerWorkload
metadata:
    name: performance-example-node
    namespace: power-manager
spec:
   name: performance-example-node
   powerProfile: performance
   nodeInfo:
     containers:
     - exclusiveCPUs:
       - 2
       - 3
       - 66
       - 67
       id: f1be89f7dda457a7bb8929d4da8d3b3092c9e2a35d91065f1b1c9e71d19bcd4f
       name: example-container
       namespace: app-namespace
       pod: example-pod
       podUID: e90ec778-006b-4bac-a954-7b022d08d5c7
       powerProfile: performance
       workload: performance-example-node
     name: example-node
     cpuIds:
     - 2
     - 3
     - 66
     - 67
````

This workload assigns the “performance” PowerProfile to cores 2, 3, 66, and 67 on the node “example-node”

The Shared PowerWorkload created by the user is determined by the Workload controller to be the designated Shared
PowerWorkload based on the AllCores value in the Workload spec. The reserved CPUs on the Node must also be specified, as
these will not be considered for frequency tuning by the controller as they are always being used by Kubernetes’
processes. It is important that the reservedCPUs value directly corresponds to the reservedCPUs value in the user’s
Kubelet config to keep them consistent. The user determines the Node for this PowerWorkload using the PowerNodeSelector
to match the labels on the Node. The user then specifies the requested PowerProfile to use.

A shared PowerWorkload must follow the naming convention of beginning with ‘shared-’. Any shared PowerWorkload that does
not begin with ‘shared-’ is rejected and deleted by the PowerWorkload controller. The shared PowerWorkload
powerNodeSelector must also select a unique node, so it is recommended that the ‘kubernetes.io/hostname’ label be used.
A shared PowerProfile can be used for multiple shared PowerWorkloads.

Creating a shared Power Workload is a necessary precondition for every node to operate Kubernetes Power Manager features properly.

### Example - Shared workload

````yaml
apiVersion: "power.amdepyc.com/v1"
kind: PowerWorkload
metadata:
  name: shared-example-node-workload
  namespace: power-manager
spec:
  name: "shared-example-node-workload"
  allCores: true
  reservedCPUs:
    - cores: [0, 1]
      powerProfile: "performance"
  powerNodeSelector:
    # Labels other than hostname can be used
    - “kubernetes.io/hostname”: “example-node”
  powerProfile: "shared"
````

### Profile Controller

The Profile Controller holds values for specific CPU settings which are then applied to cores at host level by the
Kubernetes Power Manager as requested. Power Profiles are advertised as extended resources and can be requested via the
PodSpec. The Config controller creates the requested high-performance PowerProfiles depending on which are requested in
the PowerConfig created by the user.

A base PowerProfile can be one of values:

- power
- performance
- balance-performance
- balance-power

These correspond to the EPP values. Base PowerProfiles are used to tell the Profile
controller that the specified profile is being requested for the cluster.
A PowerProfile can be requested in the PodSpec.

#### Example

````yaml
apiVersion: "power.amdepyc.com/v1"
kind: PowerProfile
metadata:
  name: performance-example-node
spec:
  name: "performance-example-node"
  max: 3700
  min: 3300
  epp: "performance"
````

Max and min values can be ommitted. Then hardware frequency limits on respective machine will be taken.
It is also possible to use percentages, where 0% = hardware minimum frequency,
100% = hardware maiximum frequency.

One or more shared PowerProfiles must be created by the user. The Power controller determines
that a PowerProfile is being designated as ‘Shared’ through the use of the ‘shared’ parameter.
This flag must be enabled when using a shared pool.

#### Example

````yaml
apiVersion: "power.amdepyc.com/v1"
kind: PowerProfile
metadata:
  name: shared-example1
spec:
  name: "shared-example1"
  max: 1500
  min: 1000
  shared: true
  epp: "power"
  governor: "powersave"
````

````yaml
apiVersion: "power.amdepyc.com/v1"
kind: PowerProfile
metadata:
  name: shared-example2
spec:
  name: "shared-example2"
  max: "90%"
  min: "10%"
  shared: true
  governor: "powersave"

````

### CPU Scaling Profile Controller

The CPU Scaling Profile Controller dynamically adjusts CPU frequency based on workload busyness,
utilizing [DPDK Telemetry](https://doc.dpdk.org/guides/howto/telemetry.html) for accurate reporting.
It leverages the userspace CPUFreq governor to manage frequency settings effectively.

When a CPU Scaling Profile is created, it generates a corresponding PowerProfile with the same name.
This PowerProfile is tasked with configuring the CPUs to operate under the userspace governor,
applying specified parameters such as EPP (Energy Performance Preference), and maximum/minimum frequency limits.

Additionally, the CPU Scaling Profile offers predefined profiles tailored to various EPP values, including:
- **power**: Optimizes for energy efficiency.
- **balance_power**: Strikes a balance between performance and power consumption.
- **balance_performance**: Prioritizes performance while maintaining reasonable power usage.
- **performance**: Maximizes performance at the cost of higher power consumption.

Table with predefined values:

| Parameter                  | Description                                           |                     |                     |                     |                     |
|----------------------------|-------------------------------------------------------|---------------------|---------------------|---------------------|---------------------|
| epp                        | Target EPP value and identifier for predefined        <br>parameter values |               power |       balance_power | balance_performance |         performance |
| cooldownPeriod             | Time to elapse after setting a new frequency target   <br>before next CPU sampling |                30ms |                30ms |                30ms |                30ms |
| targetBusyness             | Target CPU busyness, in percents                      |                  80 |                  80 |                  80 |                  80 |
| allowedBusynessDifference | Maximum difference between target and actual          <br>CPU busyness on which frequency re-evaluation <br>will not happen, in percent points |                   5 |                   5 |                   5 |                   5 |
| allowedFrequencyDifference | Maximum difference between target and actual          <br>CPU frequency on which frequency re-evaluation <br>will not happen, in MHz |                  25 |                  25 |                  25 |                  25 |
| samplePeriod               | Minimum time to elapse between two CPU sample periods |                10ms |                10ms |                10ms |                10ms |
| min                        | Minimum frequency CPUs can run at, in MHz or percents |                  0% |                  0% |                  0% |                  0% |
| max                        | Maximum frequency CPUs can run at, in MHz or percents |                100% |                100% |                100% |                100% |
| scalePercentage            | Percentage factor of CPU frequency change             <br>when scaling, in percents |                  50 |                  50 |                  50 |                  50 |


Fallback frequency for the cases when workload busyness is not available:

|               power |        balnce_power | balance_performance |         performance |
|---------------------|---------------------|---------------------|---------------------|
|                  0% |                 25% |                 50% |                100% |

The formula that is used to set CPU frequency:

$$
\text{nextTargetFrequency} = \text{currentFrequency} \times \left(1 + \left(\frac{\text{currentBusyness}}{\text{targetBusyness}} - 1\right) \times \text{scalePercentage}\right)
$$

CPU Scaling Profiles are advertised as extended resources similarly as Power Profiles and can be requested via the PodSpec.
For reference, see the Performance Pod example.

The Controller also creates CPU Scaling Configuration for nodes where the Profile is requested.

Note: CPUScalingProfile cannot be used for hosts with EPP support due to limitations for setting **userspace** CPUFreq governor.

#### Example

````yaml
apiVersion: power.amdepyc.com/v1
kind: CPUScalingProfile
metadata:
  name: cpuscalingprofile-sample
spec:
  epp: power
  samplePeriod: 20ms
  cooldownPeriod: 60ms
  targetBusyness: 80
  allowedBusynessDifference: 5
  allowedFrequencyDifference: 15
  scalePercentage: 100
  min: "100%"
  max: "0%"
````

### CPU Scaling Configuration Controller

The CPU Scaling Profile Configuration is an internal resource and should not be created or modified by end users.
The CPU Scaling Configuration Controller manages the CPU frequency for each instance where the corresponding
CPU Scaling Profile needs to be applied.

It uses DPDK Telemetry client to connect to Pod's DPDK Telemetry application. Accordingly to reported busyness,
it drives CPU frequency.

#### Example

````yaml
apiVersion: power.amdepyc.com/v1
kind: CPUScalingConfiguration
metadata:
  name: example-node
  namespace: power-manager
spec:
  items:
  - allowedBysynessDifference: 5
    allowedFrequencyDifference: 25
    cooldownPeriod: 30ms
    fallbackPercent: 25
    cpuIDs:
    - 2
    - 3
    - 8
    - 9
    podUID: e90ec778-006b-4bac-a954-7b022d08d5c7
    powerProfile: cpuscalingprofile-sample
    samplePeriod: 10ms
    scalePercentage: 50
    targetBusyness: 80
````

### PowerNode Controller

The PowerNode controller provides a window into the cluster's operations.
It exposes the workloads that are now being used, the profiles that are being used, the cores that are being used, and
the containers that those cores are associated to. Moreover, it informs the user of which Shared Pool is in use. The
Default Pool or the Shared Pool can be one of the two shared pools. The Default Pool will hold all the cores in the "
shared pool," none of which will have their frequencies set to a lower value, if there is no Shared PowerProfile
associated with the Node. The cores in the "shared pool"—apart from those reserved for Kubernetes processes (
reservedCPUs)—will be assigned to the Shared Pool and have their cores tuned by the Power Optimization Library if
a Shared PowerProfile is associated with the Node.

#### Example

````
apiVersion: power.amdepyc.com/v1
kind: PowerNode
Metadata:
  name: example-node
  namespace: power-manager
spec:
  nodeName: example-node
  powerContainers:
  - exclusiveCpus:
    - 2
    - 3
    - 8
    - 9
    id: c392f492e05fc245f77eba8a90bf466f70f19cb48767968f3bf44d7493e18e5b
    name: example-container
    namespace: example-app-namespace
    pod: example-pod
    podUID: e90ec778-006b-4bac-a954-7b022d08d5c7
    powerProfile: performance
    workload: performance-example-node
  powerProfiles:
  - 'shared: 2100000 || 400000 || '
  - 'powersave: 3709000 || 400000 || '
  - 'balance-performance: 2882000 || 2682000 || '
  - 'balance-power: 2055000 || 1855000 || '
  - 'performance: 3709000 || 3509000 || '
  powerWorkloads:
  - 'balance-performance: balance-performance || '
  - 'balance-power: balance-power || '
  - 'performance: performance || 2-3,8-9'
  - 'shared: shared || '
  - 'powersave: powersave || '
  reservedPools:
  - 2100000 || 400000 || 0-1
  sharedPool: shared || 2100000 || 400000 || 4-7,10-127
````

### C-States

To save energy on a system, you can command the CPU to go into a low-power mode. Each CPU has several power modes, which
are collectively called C-States. These work by cutting the clock signal and power from idle CPUs, or CPUs that are not
executing commands. While you save more energy by sending CPUs into deeper C-State modes, it does take more time for the
CPU to fully “wake up” from sleep mode, so there is a trade-off when it comes to deciding the depth of sleep.

#### C-State Implementation in the Power Optimization Library

The driver that is used for C-States is the cpu_idle driver. Everything associated with C-States in Linux is stored in
the /sys/devices/system/cpu/cpuN/cpuidle file or the /sys/devices/system/cpu/cpuidle file. To check the driver in use,
the user simply has to check the /sys/devices/system/cpu/cpuidle/current_driver file.

C-States have to be confirmed if they are actually active on the system. If a user requests any C-States, they need to
check on the system if they are activated and if they are not, reject the PowerConfig. The C-States are found in
/sys/devices/system/cpu/cpuN/cpuidle/stateN/.

#### C-State Ranges

````
C0      Operating State
C1      Idle
C2      Idle and power gated, deep sleep
````

#### Example

````yaml
apiVersion: power.amdepyc.com/v1
kind: CStates
metadata:
  # Replace <NODE_NAME> with the name of the node to configure the C-States on that node
  name: <NODE_NAME>
spec:
  sharedPoolCStates:
    C1: true
  exclusivePoolCStates:
    performance:
      C1: false
  individualCoreCStates:
    "3":
      C1: true
      C2: false
````

### amd-pstate CPU Performance Scaling Driver
  The amd_pstate is a part of the CPU performance scaling subsystem in the Linux kernel (CPUFreq).

  In some situations it is desirable or even necessary to run the program as fast as possible and then there is no reason to use any P-states different from the highest one (i.e. the highest-performance frequency/voltage configuration available). In some other cases, however, it may not be necessary to execute instructions so quickly and maintaining the highest available CPU capacity for a relatively long time without utilizing it entirely may be regarded as wasteful. It also may not be physically possible to maintain maximum CPU capacity for too long for thermal or power supply capacity reasons or similar. To cover those cases, there are hardware interfaces allowing CPUs to be switched between different frequency/voltage configurations or (in the ACPI terminology) to be put into different P-states.

  #### P-State Governors 
  In order to offer dynamic frequency scaling, the cpufreq core must be able to tell these drivers of a "target frequency". So these specific drivers will be transformed to offer a "->target/target_index/fast_switch()" call instead of the "->setpolicy()" call. For set_policy drivers, all stays the same, though.

### acpi-cpufreq scaling driver
  An alternative to the P-state driver is the acpi-cpufreq driver. 
  It operates in a similar fashion to the P-state driver but offers a different set of governors which can be seen [here](https://www.kernel.org/doc/html/v4.12/admin-guide/pm/cpufreq.html).
  One notable difference between the P-state and acpi-cpufreq driver is that the afformentioned scaling_max_freq value is limited to base clock frequencies rather than turbo frequencies.
  When turbo is enabled the core frequency will still be capable of exceeding base clock frequencies and the value of scaling_max_freq.

### Time Of Day

The Time Of Day feature allows users to change the configuration of their system at a given time each day. This is done
through the use of a `timeofdaycronjob`
which schedules itself for a specific time each day and gives users the option of tuning cstates, the shared pool
profile as well as the profile used by individual pods.

#### Example

```yaml
apiVersion: power.amdepyc.com/v1
kind: TimeOfDay
metadata:
  # Replace <NODE_NAME> with the name of the node to use TOD on
  name: <NODE_NAME>
  namespace: power-manager
spec:
  timeZone: "Eire"
  schedule:
    - time: "14:56"
      # this sets the profile for the shared pool
      powerProfile: balance-power
      # this transitions exclusive pods matching a given label from one profile to another
      # please ensure that only pods to be used by power manager have this label
      pods:
        - labels:
            matchLabels:
              power: "true"
          target: balance-performance
        - labels:
            matchLabels:
              special: "false"
          target: balance-performance
      # this field simply takes a cstate spec
      cState:
        sharedPoolCStates:
          C1: false
          C2: true
    - time: "23:57"
      powerProfile: shared
      cState:
        sharedPoolCStates:
          C1: true
          C2: false
      pods:
      - labels:
          matchLabels:
            power: "true"
        target: performance
      - labels:
          matchLabels:
            special: "false"
        target: balance-power
    - time: "14:35"
      powerProfile: balance-power
  reservedCPUs: [ 0,1 ]

```
The `TimeOfDay` object is deployed on a per-node basis and should have the same name as the node it's deployed on.
When applying changes to the shared pool, users must specify the CPUs reserved by the system. Additionally the user must
specify a timezone to schedule with.
The configuration for Time Of Day consists of a schedule list. Each item in the list consists of a time and any desired
changes to the system.
The `profile` field specifies the desired profile for the shared pool.
The `pods` field is used to change the profile associated with a specific pod.
To change the profile of specific pods users must provide a set of labels and profiles. When a pod matching a label is found it will be placed in a workload that matches the requested profile.
Please note that all pods matching a provided label must be configured for use with the power manager by requesting an intial profile and dedicated cores.
Finally the `cState` field accepts the spec values from a CStates configuration and applies them to the system.

### Uncore Frequency

Uncore frequency can be configured on a system-wide, per-package and per-die level. Die config will precede package,
which will in turn precede system-wide configuration.\
Valid max and min uncore frequencies are determined by the hardware

#### Example

````yaml
apiVersion: power.amdepyc.com/v1
kind: Uncore
metadata:
  name: <NODE_NAME>
  namespace: power-manager
spec:
  sysMax: 2300000
  sysMin: 1300000
  dieSelector:
    - package: 0
      die: 0
      min: 1500000
      max: 2400000
````

### In the Kubernetes API

- PowerConfig CRD
- PowerWorkload CRD
- PowerProfile CRD
- CPUScalingProfile CRD
- CPUScalingConfiguration CRD
- PowerNode CRD
- C-State CRD

If any error occurs it will be displayed in the status field of the custom resource, eg.
````yaml
apiVersion: power.amdepyc.com/v1
kind: CStates
  ...
status:
  errors:
  - the C-States CRD name must match name of one of the power nodes
````
If no errors occurred or were corrected, the list will be empty
````yaml
apiVersion: power.amdepyc.com/v1
kind: CStates
  ...
status:
  errors: []
````
### Metrics

Following metrics are provided by Power Node Agent:

| Metric name | Description | Unit | Type | Labels | Source | Note | Restriction |
| ----------- | ----------- | ---- | ---- | ------ | ------ | ---- | ----------- |
| power_perf_cycles_total | Counter of cycles on specific CPU | | counter | package, die, core, cpu | perf | Be wary of what happens during CPU frequency scaling. | |
| power_perf_instructions_total | Counter of retired instructions on specific CPU | | counter | package, die, core, cpu | perf | Be careful, these can be affected by various issues, most notably hardware interrupt counts. | |
| power_perf_stalled_cycles_frontend_total | Counter of stalled cycles during issue on specific CPU | | counter | package, die, core, cpu | perf | | |
| power_perf_bpf_output_total | Counter of BPF outputs on specific CPU | | counter | package, die, core, cpu | perf | This is used to generate raw sample data from BPF. BPF programs can write to this even using bpf_perf_event_output helper. | |
| power_perf_branch_instructions_total | Counter of retired branch instructions on specific CPU | | counter | package, die, core, cpu | perf | Prior to Linux 2.6.35, this used the wrong event on AMD processors. | |
| power_perf_branch_misses_total | Counter of mispredicted branch instructions on specific CPU | | counter | package, die, core, cpu | perf | | |
| power_perf_bus_cycles_total | Counter of bus cycles on specific CPU | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_stalled_cycles_backend_total | Counter of stalled cycles during retirement on specific CPU | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_ref_cycles_total | Counter of cycles not affected by frequency scaling on specific CPU | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_accesses_total | Counter of cache accesses on specific CPU | | counter | package, die, core, cpu | perf | Cache accesses. Usually this indicates Last Level Cache accesses but this may vary depending on your CPU. This may include prefetches and coherency messages; again this depends on the design of your CPU. | |
| power_perf_cache_bpu_read_accesses_total | Counter of branch prediction unit cache total read accesses | | counter | package, die, core, cpu | perf | | |
| power_perf_cache_bpu_read_misses_total | Counter of branch prediction unit cache total read misses | | counter | package, die, core, cpu | perf | Cache misses. Usually this indicates Last Level Cache misses; this is intended to be used in conjunction with the power_perf_cache_accesses_total event to calculate cache miss rates. | |
| power_perf_cache_bpu_write_accesses_total | Counter of branch prediction unit cache total write accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_bpu_write_misses_total | Counter of branch prediction unit cache total write misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_bpu_prefetch_accesses_total | Counter of branch prediction unit cache total prefetch accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_bpu_prefetch_misses_total | Counter of branch prediction unit cache total prefetch misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1d_read_accesses_total | Counter of level 1 data cache total read accesses | | counter | package, die, core, cpu | perf | | |
| power_perf_cache_l1d_read_misses_total | Counter of level 1 data cache total read misses | | counter | package, die, core, cpu | perf | | |
| power_perf_cache_l1d_write_accesses_total | Counter of level 1 data cache total write accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1d_write_misses_total | Counter of level 1 data cache total write misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1d_prefetch_accesses_total | Counter of level 1 data cache total prefetch accesses | | counter | package, die, core, cpu | perf | | |
| power_perf_cache_l1d_prefetch_accesses_total | Counter of level 1 data cache total prefetch misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1i_read_accesses_total | Counter of level 1 instruction cache total read accesses | | counter | package, die, core, cpu | perf | | |
| power_perf_cache_l1i_read_misses_total | Counter of level 1 instruction cache total read misses | | counter | package, die, core, cpu | perf | | |
| power_perf_cache_l1i_write_accesses_total | Counter of level 1 instruction cache total write accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1i_write_misses_total | Counter of level 1 instruction cache total write misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1i_prefetch_accesses_total | Counter of level 1 instruction cache total prefetch accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_l1i_prefetch_misses_total | Counter of level 1 instruction cache total prefetch misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_node_read_accesses_total | Counter of node cache total read accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_node_read_misses_total | Counter of node cache total read misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_node_write_accesses_total | Counter of node cache total write accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_node_write_misses_total | Counter of node cache total write misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_node_prefetch_accesses_total | Counter of node cache total prefetch accesses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_node_prefetch_misses_total | Counter of node cache total prefetch misses | | counter | package, die, core, cpu | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_ll_read_accesses_total | Counter of last level cache total read accesses | | counter | package, die | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_ll_read_misses_total | Counter of last level cache total read misses | | counter | package, die | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_ll_write_accesses_total | Counter of last level cache total write accesses | | counter | package, die | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_ll_write_misses_total | Counter of last level cache total write misses | | counter | package, die | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_ll_prefetch_accesses_total | Counter of last level cache total prefetch accesses | | counter | package, die | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_ll_prefetch_misses_total | Counter of last level cache total prefetch misses | | counter | package, die | perf | | not tested, not supported by linux kernel as of 2024/11 |
| power_perf_cache_misses_total | Counter of cache misses on specific CPU | | counter | package, die, core, cpu | perf | | |
| package_energy_consumption_joules_total | Counter of total package energy consumption in joules | joule | counter | package | perf | | |
| power_msr_c0_residency_percent | Gauge of CPU residency in C0 | percent | gauge | package, die, core, cpu | MSR | | |
| power_msr_core_energy_consumption_joules_total | Counter of total core energy consumption in joules | joule | counter | package, die, core | MSR | | |
| power_msr_cx_residency_percent | Gauge of CPU residency in C-states other than C0 | percent | gauge | package, die, core, cpu | MSR | | |
| power_msr_package_energy_consumption_joules_total | Counter of total package energy consumption in joules | joule | counter | package | MSR | | |
| power_esmi_core_energy_consumption_joules_total | Counter of total core energy consumption in joules. | joule | counter | package, die, core | E-SMI | | |
| power_esmi_data_fabric_clock_megahertz | Gauge of data fabric clock in megahertz. | megahertz | gauge | package | E-SMI | | |
| power_esmi_memory_clock_megahertz | Gauge of memory clock in megahertz. | megahertz | gauge | package | E-SMI | | |
| power_esmi_core_clock_throttle_limit_megahertz | Gauge of package core clock throttle limit in megahertz. | megahertz | gauge | package | E-SMI | | |
| power_esmi_package_frequency_limit_megahertz | Gauge of package frequency limit in megahertz. | megahertz | gauge | package | E-SMI | | |
| power_esmi_package_min_frequency_megahertz | Gauge of package minimum frequency in megahertz. | megahertz | gauge | package | E-SMI | | |
| power_esmi_package_max_frequency_megahertz | Gauge of package maximum frequency in megahertz. | megahertz | gauge | package | E-SMI | | |
| power_esmi_core_frequency_limit_megahertz | Gauge of core frequency limit in megahertz. | megahertz | gauge | package, die, core | E-SMI | | |
| power_esmi_rail_frequency_limit | Gauge of package rail frequency limit policy. | | gauge | package | E-SMI | Values: 1 = all cores on both rails have same frequency limit, 0 = each rail has different independent frequency limit. | |
| power_esmi_df_cstate_enabling_control | Gauge of package DF C-state enabling control. | | gauge | package | E-SMI | Values: 1 = DFC enabled, 0 = DFC disabled. | |
| power_esmi_package_power_watt | Gauge of package power in watts. | watt | gauge | package | E-SMI | | |
| power_esmi_package_power_cap_watt | Gauge of package power cap in watts. | watt | gauge | package | E-SMI | | |
| power_esmi_package_power_max_cap_watt | Gauge of package power max cap in watts. | watt | gauge | package | E-SMI | | |
| power_esmi_power_efficiency_mode | Gauge of package power efficiency mode. | | gauge | package | E-SMI | Values: 0 = high performance mode, 1 = power efficient mode, 2 = IO performance mode, 3 = balanced memory performance mode, 4 = balanced core performance mode, 5 = balanced core and memory performance mode. | |
| power_esmi_core_boost_limit_megahertz | Gauge of core frequency boost limit in megahertz. | megahertz | gauge | package, die, core | E-SMI | | |
| power_esmi_c0_residency_percent | Gauge of package residency in C0. | percent | gauge | package | E-SMI | | |
| power_esmi_package_temperature_celsius | Gauge of package temperature in degree Celsius. | degree celsius | gauge | package | E-SMI | | |
| power_esmi_ddr_bandwidth_utilization_gigabytes_per_second | Gauge of DDR bandwidth utilization in GB/s. | gb/s | gauge | package | E-SMI | | |
| power_esmi_ddr_bandwidth_utilization_percent | Gauge of DDR bandwidth utilization in percent of max bandwidth. | percent | gauge | package | E-SMI | | |
| power_esmi_dimm_power_watts | Gauge of DIMM power in watts. | watt | gauge | | E-SMI | | |
| power_esmi_dimm_temperature_celsius | Gauge of DIMM temperature in degree Celsius. | degree celsius | gauge | | E-SMI | | |
| power_esmi_lclk_dpm_min_level | gauge of minimum lclk dpm level. | | gauge | | e-smi | Values: either 0 or 1 | |
| power_esmi_lclk_dpm_max_level | Gauge of maximum LCLK DPM level. | | gauge | | E-SMI | Values: either 0 or 1 | |
| power_esmi_io_link_bandwidth_utilization_megabits_per_second | Gauge of IO link bandwidth utilization in Mb/s. | mb/s | gauge | | E-SMI | | |
| power_esmi_xgmi_aggregate_bandwidth_utilization_megabits_per_second | Gauge of xGMI aggregate bandwidth utilization in Mb/s. | mb/s | gauge | | E-SMI | | |
| power_esmi_xgmi_read_bandwidth_utilization_megabits_per_second | Gauge of xGMI read bandwidth utilization in Mb/s. | mb/s | gauge | | E-SMI | | |
| power_esmi_xgmi_write_bandwidth_utilization_megabits_per_second | Gauge of xGMI write bandwidth utilization in Mb/s. | mb/s | gauge | | E-SMI | | |
| power_esmi_processor_family | Gauge of processor family. | | gauge | | E-SMI | | |
| power_esmi_processor_model | Gauge of processor model. | | gauge | | E-SMI | | |
| power_esmi_cpus_per_core | Gauge of CPUs per core. | | gauge | | E-SMI | | |
| power_esmi_cpus | Gauge of total number of CPUs in the system. | | gauge | | E-SMI | | |
| power_esmi_packages | Gauge of total number of packages in the system. | | gauge | | E-SMI | | |
| power_esmi_smu_firmware_major_version | Gauge of SMU firmware major version. | | gauge | | E-SMI | | |
| power_esmi_smu_firmware_minor_version | Gauge of SMU firmware minor version. | | gauge | | E-SMI | | |

#### Telegraf configuration for metrics scraping

Metrics are provided in Prometheus exposition format. Metrics endpoint is exposed by Power Node Agent Pod and is accessible from Pods residing on the same worker node using the following URL:
http://node-agent-metrics-local-service.power-manager.svc.cluster.local:10001/metrics

For metrics scraping, it is recommended to deploy [Telegraf](https://github.com/influxdata/telegraf) as a Pod on each worker node that is required to collect metrics. Prometheus input plugin can be used,
with configured endpoint:

````
urls = ["http://node-agent-metrics-local-service.power-manager.svc.cluster.local:10001/metrics"]
````

#### Prometheus configuration for metrics scraping

Prometheus can scrape metrics from Power Node Agent either via Telegraf or directly via metrics port.
In case that Prometheus is deployed as an Kubernetes Operator, metrics scraping can be set up using PodMonitor.
Example of PodMonitor manifest:

````
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: node-agent-monitor
  namespace: monitoring   # same namespace as prometheus operator
  labels:
    release: prometheus-operator  # same as release label of prometheus operator
spec:
  namespaceSelector:
    matchNames:
    - power-manager
  podMetricsEndpoints:
  - interval: 30s
    path: /metrics
    port: metrics
    scheme: http
  selector:
    matchLabels:
      name: power-node-agent-pod
````

In case that Prometheus is not deployed as a Kubernetes Operator, metrics scraping can be configured using [Pod role](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#pod).

### Node Agent Pod

The Pod Controller watches for pods. When a pod comes along the Pod Controller checks if the pod is in the guaranteed
quality of service class (using exclusive
cores, [see documentation](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/), taking a core
out of the shared pool (it is the only option in Kubernetes that can do this operation). Then it examines the Pods to
determine which PowerProfile has been requested and then creates or updates the appropriate PowerWorkload.

Note: the request and the limits must have a matching number of cores and are also in a container-by-container bases.
Currently the Kubernetes Power Manager only supports a single PowerProfile per Pod. If two profiles are requested in
different containers, the pod will get created but the cores will not get tuned.

## Installation

## Step by step build

### Setting up the Kubernetes Power Manager

- Clone the Kubernetes Power Manager

````
git clone https://github.com/amdepyc/kubernetes-power-manager
cd kubernetes-power-manager
````

- Set up the necessary Namespace, Service Account, and RBAC rules for the Kubernetes Power Manager:

````
kubectl apply -f config/rbac/namespace.yaml
kubectl apply -f config/rbac/rbac.yaml
````

- Generate the CRD templates, create the Custom Resource Definitions, and install the CRDs:

````
make
````

- Docker Images
  Docker images can either be built locally by using the command:

````
make images
````
or available by pulling from the AMD's public Docker Hub at:
 - amdepyc/power-operator:TAG 
 - amdepyc/power-node-agent:TAG

### Running the Kubernetes Power Manager

- **Applying the manager**

Apply the manager:
`kubectl apply -f config/manager/manager.yaml`

The controller-manager-xxxx-xxxx pod will be created.

- **Power Config**

The example PowerConfig in examples/example-powerconfig.yaml contains the following PowerConfig spec:

````yaml
apiVersion: "power.amdepyc.com/v1"
kind: PowerConfig
metadata:
  name: power-config
spec:
  powerNodeSelector:
    feature.node.kubernetes.io/power-node: "true"
  powerProfiles:
    - "performance"
 ````

Apply the Config:
`kubectl apply -f examples/example-powerconfig.yaml`

Once deployed the controller-manager pod will see it via the Config controller and create a Node Agent instance on nodes
specified with the ‘feature.node.kubernetes.io/power-node: "true"’ label.

The power-node-agent DaemonSet will be created, managing the Power Node Agent Pods. The controller-manager will finally
create the PowerProfiles that were requested on each Node.

- **Shared Profile**

The example Shared PowerProfile in examples/example-shared-profile.yaml contains the following PowerProfile spec:

````yaml
apiVersion: power.amdepyc.com/v1
kind: PowerProfile
metadata:
  name: shared
  namespace: power-manager
spec:
  name: "shared"
  max: 1000
  min: 1000
  epp: "power"
  shared: true
  governor: "powersave"
````

Apply the Profile:
`kubectl apply -f examples/example-shared-profile.yaml`

- **Shared workload**

The example Shared PowerWorkload in examples/example-shared-workload.yaml contains the following PowerWorkload spec:

````yaml
apiVersion: power.amdepyc.com/v1
kind: PowerWorkload
metadata:
  # Replace <NODE_NAME> with the Node associated with PowerWorkload 
  name: shared-<NODE_NAME>-workload
  namespace: power-manager
spec:
  # Replace <NODE_NAME> with the Node associated with PowerWorkload 
  name: "shared-<NODE_NAME>-workload"
  allCores: true
  reservedCPUs:
    # IMPORTANT: The CPUs in reservedCPUs should match the value of the reserved system CPUs in your Kubelet config file
    - cores: [0, 1]
  powerNodeSelector:
    # The label must be as below, as this workload will be specific to the Node
    kubernetes.io/hostname: <NODE_NAME>
  # Replace this value with the intended shared PowerProfile
  powerProfile: "shared"

````

Replace the necessary values with those that correspond to your cluster and apply the Workload:
`kubectl apply -f examples/example-shared-workload.yaml`

Once created the workload controller will see its creation and create the corresponding Pool. All of the cores on the
system except the reservedCPUs will then be brought down to this lower frequency level. 
The reservedCPUs will be kept at the system default min and max frequency by default. If the user specifies a profile along with a set of reserved 
cores then a separate pool will be created for those cores and that profile. If an invalid profile is supplied the cores will instead be placed in
the default reserved pool with system defaults. It should be noted that in most instances leaving these cores at system defaults is the best approach 
to prevent important k8s or kernel related processes from becoming starved.

- **CPU Scaling Profile**

The example CPU Scaling Profile in examples/example-cpuscalingprofile.yaml contains following PodSpec:

````yaml
apiVersion: power.amdepyc.com/v1
kind: CPUScalingProfile
metadata:
  name: cpuscalingprofile-sample
spec:
  samplePeriod: 20ms
  cooldownPeriod: 60ms
  targetBusyness: 80
  allowedBusynessDifference: 5
  allowedFrequencyDifference: 15
  scalePercentage: 100
  min: "100%"
  max: "0%"
  epp: power
````

to apply the CPU Scaling Profile:
`kubectl apply -f examples/example-cpuscalingprofile.yaml`

- **Performance Pod**

The example Pod in examples/example-pod.yaml contains the following PodSpec:

````yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-power-pod
spec:
  containers:
    - name: example-power-container
      image: ubuntu
      command: [ "/bin/sh" ]
      args: [ "-c", "sleep 15000" ]
      resources:
        requests:
          memory: "200Mi"
          cpu: "2"
          # Replace <POWER_PROFILE> with the PowerProfile you wish to request
          # IMPORTANT: The number of requested PowerProfiles (or CPUScalingProfiles) must match the number of requested CPUs
          # IMPORTANT: If they do not match, the Pod will be successfully scheduled, but the PowerWorkload for the Pod will not be created
          power.amdepyc.com/<POWER_OR_CPUSCALING_PROFILE>: "2"
        limits:
          memory: "200Mi"
          cpu: "2"
          # Replace <POWER_PROFILE> with the PowerProfile you wish to request
          # IMPORTANT: The number of requested PowerProfiles must match the number of requested CPUs
          # IMPORTANT: If they do not match, the Pod will be successfully scheduled, but the PowerWorkload for the Pod will not be created
          power.amdepyc.com/<POWER_OR_CPUSCALING_PROFILE>: "2"
````

Replace the placeholder values with the PowerProfile you require and apply the PodSpec:

`kubectl apply -f examples/example-pod.yaml`

At this point, if only the ‘performance’ PowerProfile was selected in the PowerConfig, the user’s cluster will contain
two PowerWorkloads:

`kubectl get powerworkloads -n power-manager`

````
NAME                                   AGE
performance-<NODE_NAME>-workload       63m
shared-<NODE_NAME>-workload            61m
````

- **Performance Pod with sample DPDK application**

A precondition for this example is to have 1GiB hugepages configured on target nodes. This can be verified by:

`kubectl get node <NODE_NAME> -o custom-columns=NAME:.metadata.name,HUGEPAGES-1GI:.status.allocatable."hugepages-1Gi" `

Refer to the [Kubernetes hugepage documentation](https://kubernetes.io/docs/tasks/manage-hugepages/scheduling-hugepages/) for more details.

The example Pod with sample DPDK application in examples/example-dpdk-testapp.yaml contains the following PodSpec:

````yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dpdk-testapp
  namespace: power-manager
  labels:
    app: dpdk-testapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: dpdk-testapp
  template:
    metadata:
      labels:
        app: dpdk-testapp
    spec:
      nodeSelector:
        "feature.node.kubernetes.io/power-node": "true"
      containers:
        - name: server
          env:
            - name: POD_UID
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: metadata.uid
          image: amdepyc/dpdk-testapp:0.0.1
          imagePullPolicy: Always # IfNotPresent
          command: ["tail", "-f", "/dev/null"]
          securityContext:
            privileged: true
          resources:
            requests:
              cpu: "32"
              hugepages-1Gi: "10Gi"
              memory: "6Gi"
              power.amdepyc.com/cpuscalingprofile-sample: "32"
            limits:
              cpu: "32"
              hugepages-1Gi: "10Gi"
              memory: "6Gi"
              power.amdepyc.com/cpuscalingprofile-sample: "32"
          volumeMounts:
            - mountPath: /hugepages-1Gi
              name: hugepages-1gi
            - mountPath: /var/run/memif
              name: memif
            - name: pods
              mountPath: /var/run/dpdk/rte
              subPathExpr: $(POD_UID)/dpdk/rte
        - name: client
          env:
            - name: POD_UID
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: metadata.uid
          image: amdepyc/dpdk-testapp:0.0.1
          imagePullPolicy: Always
          command: ["tail", "-f", "/dev/null"]
          securityContext:
            privileged: true
          resources:
            requests:
              cpu: "8"
              hugepages-1Gi: "5Gi"
              memory: "3Gi"
              power.amdepyc.com/performance: "8"
            limits:
              cpu: "8"
              hugepages-1Gi: "5Gi"
              memory: "3Gi"
              power.amdepyc.com/performance: "8"
          volumeMounts:
            - mountPath: /hugepages-1Gi
              name: hugepages-1gi
            - mountPath: /var/run/memif
              name: memif
            - name: pods
              mountPath: /var/run/dpdk/rte
              subPathExpr: $(POD_UID)/dpdk/rte
      volumes:
        - name: hugepages-1gi
          emptyDir:
            medium: HugePages-1Gi
        - name: memif
          emptyDir: {}
        - name: pods
          hostPath:
            path: /var/lib/power-node-agent/pods
            type: DirectoryOrCreate
````

To apply the PodSpec, run:
`kubectl apply -f examples/example-pod.yaml`

- **Delete Pods**

`kubectl delete pods <name>`

When a Pod that was associated with a PowerWorkload is deleted, the cores associated with that Pod will be removed from
the corresponding PowerWorkload. If that Pod was the last requesting the use of that PowerWorkload, the workload will be
deleted. All cores removed from the PowerWorkload are added back to the Shared PowerWorkload for that Node and returned
to the lower frequencies.

