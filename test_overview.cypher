MATCH (n) WHERE n:Domain OR (n:Feature AND NOT ()-[]->(n)) RETURN n
