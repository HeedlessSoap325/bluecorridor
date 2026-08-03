# BlueCorridor

> *A "blue corridor" is an ocean superhighway for marine life, for example whales, when they migrate*

---

## Motivation

I have a few Docker Containers and Compose Projects, with a lot of volumes and custom networks. It is very time-consuming to migrate all these containers manually, and get them to work with the right configuration. BlueCorridor is a project aimed to solve that issue by simply creating one file containing all the data, that can then be easily copied and re-imported.

---

## Tech Stack

- **[Go](https://go.dev)**: The Go programming language

## Getting Started

### Option A - Pre-Built executable

#### Steps

**1. Download the executable for your system**

**2. Give the programm execution permission**

This step is only necessary if you are on a **linux based** operating system

```shell
chmod u+x ./bluecorridor-{version}-{platform}-{architecture}
```

**3. Run the command**

```shell
./bluecorridor-{version}-{platform}-{architecture}[.exe] export
```

for help:

```shell
./bluecorridor-{version}-{platform}-{architecture}[.exe] help
```

---

### Option B - Run from source

#### Prerequisites

- A compatible version of [Go](https://go.dev) installed on your system

#### Steps

**1. Clone the repository**

```shell
git clone https://github.com/HeedlessSoap325/bluecorridor.git
```

**2. navigate to the source directory**

```shell
cd bluecorridor/src
```

**3. Run the project**

```shell
go run main.go export
```

for help:

```shell
go run main.go help
```