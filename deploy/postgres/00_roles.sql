-- Pokreće se kao superuser pri inicijalizaciji klastera (docker-entrypoint-initdb.d).
-- Namerno NIJE u migracijama: migracije ne smeju da sadrže lozinke.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE ROLE aip_migrator LOGIN PASSWORD 'migrator';
CREATE ROLE aip_app LOGIN PASSWORD 'app' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;

GRANT CONNECT ON DATABASE aip TO aip_migrator, aip_app;
GRANT USAGE, CREATE ON SCHEMA public TO aip_migrator;
GRANT USAGE ON SCHEMA public TO aip_app;

-- tabele koje migrator napravi u budućnosti automatski dobijaju prava za app
ALTER DEFAULT PRIVILEGES FOR ROLE aip_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO aip_app;
ALTER DEFAULT PRIVILEGES FOR ROLE aip_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO aip_app;
