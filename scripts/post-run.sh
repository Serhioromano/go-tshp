apt update
apt upgrade -y
git config --global user.name "Serhioromano"
git config --global user.email "serg4172@mail.ru"
apt install sqlite3

apt-get purge golang*
cd ~
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz

echo "export PATH=$PATH:/usr/local/go/bin" >> .bashrc
echo "export PATH=$PATH:/usr/local/go/bin" >> .profile
source .bashrc
source .profile

npm install
