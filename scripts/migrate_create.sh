#!/bin/bash
read -p "Введите имя миграции (snake_case): " name
if [ -z "$name" ]; then
  echo "Ошибка: имя миграции не может быть пустым"
  exit 1
fi
./bin/migrate create -ext sql -dir ./migrations "$name"
