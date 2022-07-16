CREATE TABLE nodenames(
       node_id STRING NOT NULL,
       name STRING NOT NULL,
       PRIMARY KEY(node_id));
CREATE TABLE links(
       router STRING NOT NULL,
       net STRING NOT NULL,
       cost INT NOT NULL,
       PRIMARY KEY(router, net));
CREATE TABLE maps(
       map_id STRING NOT NULL,
       name STRING NOT NULL,
       PRIMARY KEY(map_id));
CREATE TABLE pos(
       map_id STRING NOT NULL REFERENCES maps(map_id),
       node_id STRING NOT NULL,
       x INT NOT NULL,
       y INT NOT NULL,
       PRIMARY KEY(map_id, node_id));
