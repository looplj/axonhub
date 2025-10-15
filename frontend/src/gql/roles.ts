export const ROLES_QUERY = `
  query Roles($first: Int, $after: Cursor, $where: RoleWhereInput) {
    roles(first: $first, after: $after, where: $where) {
      edges {
        node {
          id
          name
          code
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
  query AllScopes {
    allScopes {
      scope
      description
    }
  }
`;