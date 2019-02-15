<p align="center" background="black"><img src="minter-logo.svg" width="400"></p>

<p align="center" style="text-align: center;">
    <a href="https://github.com/daniildulin/explorer-gate/blob/master/LICENSE">
        <img src="https://img.shields.io/packagist/l/doctrine/orm.svg" alt="License">
    </a>
    <img alt="undefined" src="https://img.shields.io/github/last-commit/MinterTeam/minter-explorer-extender.svg">
</p>

# Minter Explorer Extender

The official repository of Minter Explorer Extender service.

_NOTE: This project in active development stage so feel free to send us questions, issues, and wishes_


## RUN

Use flags or environment variables to run service. Flags have higher priority

| Flag          | Environment           | Description             |
|:--------------|:----------------------|:------------------------|
| db_name       | EXPLORER_DB_NAME      | Name of database        |
| db_user       | EXPLORER_DB_USER      | Database user           |
| db_password   | EXPLORER_DB_PASSWORD  | Database password       |
| node_api      | MINTER_NODE_API       | Minter node url         |
| tx_chunk_size | -                     | Transactions chunk  ize |


Examples:

./extender -db_password=password -db_user=minter -db_name=explorer -node_api=http://127.0.0.1:8841

EXPLORER_DB_PASSWORD=password EXPLORER_DB_USER=minter EXPLORER_DB_NAME=explorer MINTER_NODE_API=http://127.0.0.1:8841 ./extender
