# Ansible

## install tools

```console
$ ansible-playbook -i ./ansible/hosts.yml ./ansible/playbook/install_tools.yml --verbose
```

## before bench

for specific:

```console
$ ansible-playbook -i ./ansible/hosts.yml -l isucon-1 ./ansible/playbook/before_bench.yml --extra-vars "env=dev" --extra-vars "branch=main" --verbose
$ ansible-playbook -i ./ansible/hosts.yml -l isucon-1 ./ansible/playbook/before_bench.yml --extra-vars "env=prod" --extra-vars "branch=main" --verbose
```

for all:

```console
$ ansible-playbook -i ./ansible/hosts.yml ./ansible/playbook/before_bench.yml --extra-vars "env=dev" --extra-vars "branch=main" --verbose
$ ansible-playbook -i ./ansible/hosts.yml ./ansible/playbook/before_bench.yml --extra-vars "env=prod" --extra-vars "branch=main" --verbose
```

## after_bench

```console
$ ansible-playbook -i ./ansible/hosts.yml -l isucon-1 ./ansible/playbook/after_bench.yml --verbose
```

## sandbox

```console
$ ansible-playbook -i ./ansible/hosts.yml -l isucon-1 ./ansible/playbook/sandbox.yml --verbose
```
