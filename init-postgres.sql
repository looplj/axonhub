-- PostgreSQL Database Initialization Script for AxonHub

-- Create database (run this as postgres superuser)
CREATE DATABASE axonhub;

-- Connect to the database
\c axonhub;

-- Optional: Create a dedicated user for AxonHub
-- CREATE USER axonhub_user WITH PASSWORD 'your_secure_password';
-- GRANT ALL PRIVILEGES ON DATABASE axonhub TO axonhub_user;

-- The tables will be created automatically by Ent ORM when the application starts
