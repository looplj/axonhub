interface MemberProject {
  projectID: string;
}

interface MeData {
  projects?: MemberProject[] | null;
}

export function isProjectSelectionValid(
  meData: MeData | null | undefined,
  selectedProjectId: string | null | undefined,
): boolean {
  if (!meData) {
    return false;
  }
  if (!selectedProjectId) {
    return true;
  }
  return (meData.projects ?? []).some((p) => p.projectID === selectedProjectId);
}