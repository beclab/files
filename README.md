# files

A file manager for Terminus

## Installation

- Install node in you system, You can check if you have node installed by running this code in your terminal
  ```bash
  node -v
  ```
  
  if it's not there, you can install from this site <a href="https://nodejs.org/en/" target="_blank"> Node JS</a>

- Clone the repo 
  ```bash
  git clone https://github.com/beclab/files.git
  ```

- Navigate to the `files/` folder
  Then start the server
  ```bash
  ./filebrowser --noauth
  ```
  ```bash
  # Output:
  Listening on 127.0.0.1:8110
  ```
  
- Navigate to the `frontend/` folder
  ```bash
  cd packages/frontend
  ```
  Then install all the dependencies
  ```bash
  npm i
  ```
  Then start the server
  ```bash
  quasar dev
  ```
  ```bash
  #Output:
  Opening default browser at http://localhost:8100/
  ```
