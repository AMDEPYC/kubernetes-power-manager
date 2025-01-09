#!/usr/bin/env python3

import glob
import os
import re
import sys
import socket
import json
import curses
import time
import struct
import argparse

TOPOPATH = "/sys/devices/system/cpu"


def parse_intervals(interval_string):
    intervals = interval_string.split(',')
    result = []

    for interval in intervals:
        if '-' in interval:
            start, end = map(int, interval.split('-'))
            result.extend(range(start, end + 1))
        else:
            result.append(int(interval))

    return result


def connect_to_socket(socket_path):
    try:
        client_socket = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
        client_socket.connect(socket_path)
        response = client_socket.recv(16384)
        return client_socket
    except Exception as e:
        return None

def send_command(client_socket, command):
    try:
        client_socket.send(command.encode())
        response = client_socket.recv(16384)
        return json.loads(response.decode())
    except Exception as e:
        return None

def read_busyness(socket):
    client_socket = connect_to_socket(socket)
    if not client_socket:
        return None
    command = "/eal/lcore/busyness"
    response = send_command(client_socket, command)
    client_socket.close()
    return response

def read_msr(cpu, msr_address):
    try:
        fd = os.open(f"/dev/cpu/{cpu}/msr", os.O_RDONLY)
        os.lseek(fd, msr_address, os.SEEK_SET)
        value = struct.unpack("Q", os.read(fd, 8))[0]
        os.close(fd)
        return value
    except (FileNotFoundError, OSError) as e:
        raise Exception(f"Error accessing MSR: {e}")

class Cpu(object):
    def __init__(self, id):
        self.id = id
        self.core_id = -1
        self.cluster_id = -1
        self.die_id = -1
        self.package_id = -1
        self.core = None
        self.cluster = None
        self.die = None
        self.package = None
        self.scaling_governor = None
        self.scaling_setspeed = None
        self.scaling_cur_freq = None
        self.scaling_min_freq = None
        self.scaling_max_freq = None
        self.busyness = None
        self.utilization = None
        self.energy_counter = None
        self.current_power = None
        self.last_energy_time = None
        self.last_tsc = None
        self.last_mperf = None
        self.c0 = None


    def _read_param(self, param):
        param_file = os.path.join(TOPOPATH, "cpu" + str(self.id), param)
        with open(param_file, 'r') as file:
            content = file.read().strip()
        return content

    def _read_topology_param(self, param):
        return self._read_param(os.path.join("topology", param))

    def _read_cpufreq_param(self, param):
        return self._read_param(os.path.join("cpufreq", param))

    def read_package(self):
        self.package_id = int(self._read_topology_param('physical_package_id'))

    def read_die(self):
        self.die_id = int(self._read_topology_param('die_id'))

    def read_cluster(self):
        self.cluster_id = int(self._read_topology_param('cluster_id'))

    def read_core(self):
        self.core_id = int(self._read_topology_param('core_id'))

    def read_topology(self):
        self.read_package()
        self.read_die()
        self.read_cluster()
        self.read_core()

    def read_cpufreq(self):
        self.scaling_governor = self._read_cpufreq_param("scaling_governor")
        if self.scaling_governor == "userspace":
            self.scaling_setspeed = int(self._read_cpufreq_param("scaling_setspeed"))
        else:
            self.scaling_setspeed = 0
        self.scaling_cur_freq = int(self._read_cpufreq_param("scaling_cur_freq"))
        self.scaling_min_freq = int(self._read_cpufreq_param("scaling_min_freq"))
        self.scaling_max_freq = int(self._read_cpufreq_param("scaling_max_freq"))

    def read_energy(self):
        energy_counter_raw = read_msr(self.id, 0xC001029A)
        energy_unit = read_msr(self.id, 0xC0010299)
        energy_bits = 0b0001111100000000
        exponent = (energy_unit & energy_bits) >> 8
        energy_multiplier = 1 / (2 ** exponent)
        energy_counter = energy_counter_raw * energy_multiplier
        tm = time.time()
        if self.energy_counter:
            self.current_power = (energy_counter - self.energy_counter) / (tm - self.last_energy_time) 
        self.last_energy_time = tm
        self.energy_counter = energy_counter

    def read_c0_residency(self):
        tsc = read_msr(self.id, 0x10)
        mperf = read_msr(self.id, 0xe7)

        if self.last_tsc:
            self.c0 = (mperf - self.last_mperf) / (tsc - self.last_tsc)

        self.last_tsc = tsc
        self.last_mperf = mperf

    def get_siblings(self):
        return self.core.cpus

class Core(object):
    def __init__(self, id):
        self.id = id
        self.cpus = {}
        self.cluster = None
        self.die = None
        self.package = None

class Cluster(object):
    def __init__(self, id):
        self.id = id
        self.cpus = {}
        self.cores = {}
        self.die = None
        self.package = None

class Die(object):
    def __init__(self, id):
        self.id = id
        self.cpus = {}
        self.cores = {}
        self.clusters = {}
        self.package = None

class Package(object):
    def __init__(self, id):
        self.id = id
        self.cpus = {}
        self.dies = {}

class CpuTopology(object):
    def __init__(self, pod_uid=None):
        self.cpus = {}
        self.cores = set()
        self.clusters = set()
        self.dies = set()
        self.packages = {}
        self.pod_uid = pod_uid

    def get_or_create_core(self, core_id, cluster_id, die_id, package_id):
        core = next((c for c in self.cores if c.id == core_id and c.cluster.id == cluster_id and c.die.id == die_id and c.package.id == package_id), None)
        if not core:
            core = Core(core_id)
            self.cores.add(core)
        return core

    def get_or_create_cluster(self, cluster_id, die_id, package_id):
        cluster = next((c for c in self.clusters if c.id == cluster_id and c.die.id == die_id and c.package.id == package_id), None)
        if not cluster:
            cluster = Cluster(cluster_id)
            self.clusters.add(cluster)
        return cluster

    def get_or_create_die(self, die_id, package_id):
        die = next((d for d in self.dies if d.id == die_id and d.package.id == package_id), None)
        if not die:
            die = Die(die_id)
            self.dies.add(die)
        return die

    def get_or_create_package(self, package_id):
        package = self.packages.get(package_id)
        if not package:
            package = Package(package_id)
            self.packages[package_id] = package
        return package

    def read_topology(self):
        cpu_dirs = glob.glob(os.path.join(TOPOPATH, "cpu[0-9]*"))
        self.cpus = {(cpu_id := int(re.search(r'\d+$', cpu_dir).group())) : Cpu(cpu_id) for cpu_dir in cpu_dirs}
        for cpu in self.cpus.values():
            cpu.read_topology()

            core = self.get_or_create_core(cpu.core_id, cpu.cluster_id, cpu.die_id, cpu.package_id)
            cluster = self.get_or_create_cluster(cpu.cluster_id, cpu.die_id, cpu.package_id)
            die = self.get_or_create_die(cpu.die_id, cpu.package_id)
            package = self.get_or_create_package(cpu.package_id)

            cpu.core = core
            cpu.cluster = cluster
            cpu.die = die
            cpu.package = package

            core.cpus.setdefault(cpu.id, cpu)
            core.cluster = cluster
            core.die = die
            core.package = package
            
            cluster.cpus.setdefault(cpu.id, cpu)
            cluster.cores.setdefault(core.id, core)
            cluster.die = die
            cluster.package = package

            die.cpus.setdefault(cpu.id, cpu)
            die.cores.setdefault(core.id, core)
            die.clusters.setdefault(cluster.id, cluster)
            die.package = package

            package.cpus.setdefault(cpu.id, cpu)
            package.dies.setdefault(die.id, die)

    def read_busyness(self):
        if not self.pod_uid:
            return
        dpdk_busyness = read_busyness(os.path.join("/var/lib/power-node-agent/pods", self.pod_uid, "dpdk/rte/dpdk_telemetry.v2"))
        if not dpdk_busyness:
            for cpu in self.cpus.values():
                cpu.busyness = None
            return
        for cpu_id, busyness in dpdk_busyness['/eal/lcore/busyness'].items():
            self.cpus[int(cpu_id)].busyness = busyness

class CpuParam(object):
    def __init__(self, header, width, fmt, identifier, core_only=False):
        self.header = header
        self.width = width
        self.fmt = fmt
        self.identifier = identifier
        self.core_only = core_only

class CpuPresenter(object):
    def __init__(self, scr, params):
        self.scr = scr
        self.params = params

    def print_header(self, row, color_pair):
        column = 0
        for param in self.params:
            fmt = "{:" + param.header[0] + str(param.width) + "}"
            header = fmt.format(param.header[1:])
            self.scr.addstr(row, column, header, curses.color_pair(color_pair))
            if param.width >= len(header):
                self.scr.addstr(row, column + len(header), " " * (param.width - len(header) + 1), curses.color_pair(color_pair))
            column += param.width + 1
        self.scr.addstr(row+1, 0, "=" * column, color_pair)

    def print_cpu(self, row, cpu, color_pair):
        column = 0
        for param in self.params:
            val = getattr(cpu, param.identifier, None)
            if val is not None:
                val_present = eval(f"f'''{param.fmt}'''")
            else:
                val_present = " "
            self.scr.addstr(row, column, val_present, curses.color_pair(color_pair))
            if param.width >= len(val_present):
                self.scr.addstr(row, column + len(val_present), " " * (param.width - len(val_present) + 1), curses.color_pair(color_pair))
            column += param.width + 1

def main(stdscr, args):
    refresh_interval_sec = args.interval
    monitor_cpus = parse_intervals(args.cpu)
    monitor_siblings = not args.no_siblings
    monitor_dpdk_telemetry = args.dpdk_pod_uid != ""
    dpdk_pod_uid = args.dpdk_pod_uid
    topology = CpuTopology(dpdk_pod_uid)
    topology.read_topology()

    cpu_presenter = CpuPresenter(stdscr, [
        CpuParam("<CPU", 6, "cpu{val:<3}", "id"),
        CpuParam(">Freq", 4, "{val//1000:>4}", "scaling_cur_freq"),
        CpuParam(">Governor", 12, "{val:>12}", "scaling_governor"),
        CpuParam(">fTgt", 4, "{val//1000:>4}", "scaling_setspeed"),
        CpuParam(">fMin", 4, "{val//1000:>4}", "scaling_min_freq"),
        CpuParam(">fMax", 4, "{val//1000:>4}", "scaling_max_freq"),
        CpuParam(">Bs", 4, "{val:>4}", "busyness"),
        CpuParam(">C0%", 3, "{val*100:>3.0f}", "c0"),
        CpuParam(">mW", 5, "{val*1000:>6.0f}", "current_power", core_only=True),
        ])

    curses.start_color()
    curses.init_pair(1, curses.COLOR_WHITE, curses.COLOR_BLACK)
    curses.init_pair(2, curses.COLOR_YELLOW, curses.COLOR_BLACK)
    curses.init_pair(3, curses.COLOR_BLACK, curses.COLOR_WHITE)
    stdscr.clear()

    cpu_presenter.print_header(0, 1)
    stdscr.refresh()

    while True:
        if monitor_dpdk_telemetry:
            topology.read_busyness()
        row = 2
        for cpu_id in monitor_cpus:
            cpu = topology.cpus[cpu_id]
            cpu.read_cpufreq()
            cpu.read_energy()
            cpu.read_c0_residency()
            cpu_presenter.print_cpu(row, cpu, 3)
            row += 1
            if monitor_siblings:
                for sibling in cpu.get_siblings().values():
                    if sibling.id != cpu.id:
                        sibling.read_cpufreq()
                        sibling.read_c0_residency()
                        cpu_presenter.print_cpu(row, sibling, 2)
                        row += 1
        stdscr.refresh()
        time.sleep(refresh_interval_sec)

if __name__=="__main__":
    parser = argparse.ArgumentParser(description="Kubernetes Power Manager CPU monitor")
    parser.add_argument("--interval", "-n", type=float, default=1.0, help="refresh interval in seconds")
    parser.add_argument("--cpu", "-c", type=str, default="0", help="List of cpus to watch")
    parser.add_argument("--no-siblings", "-s", action="store_true", help="Don't watch siblings")
    parser.add_argument("--dpdk-pod-uid", "-d", type=str, default="", help="Watch dpdk busyness using dpdk application pod")
    args = parser.parse_args()
    curses.wrapper(main, args)
