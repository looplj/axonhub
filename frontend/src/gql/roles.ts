export const ROLES_QUERY = `
  query Roles($first: Int, $after: Cursor, $where: RoleWhereInput) {
    roles(first: $first, after: $after, where: $where) {
      edges {
        node {
          id
          name
          scopes
        }
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
    }
  }
`;

export const ALL_SCOPES_QUERY = `
  query AllScopes($level: String) {
    allScopes(level: $level) {
      scope
      description
      levels
    }
  }
`;