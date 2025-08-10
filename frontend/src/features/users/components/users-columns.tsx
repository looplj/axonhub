"use client";

import { ColumnDef } from "@tanstack/react-table";
import { useTranslation } from "react-i18next";
import { Checkbox } from "@/components/ui/checkbox";
import LongText from "@/components/long-text";
import { Badge } from "@/components/ui/badge";
import { User } from "../data/schema";
import { DataTableRowActions } from "./data-table-row-actions";

export const createColumns = (t: ReturnType<typeof useTranslation>['t']): ColumnDef<User>[] => [
  {
    id: "select",
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected() ||
          (table.getIsSomePageRowsSelected() && "indeterminate")
        }
        onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
        aria-label="Select all"
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        onCheckedChange={(value) => row.toggleSelected(!!value)}
        aria-label="Select row"
      />
    ),
    enableSorting: false,
    enableHiding: false,
  },
  {
    accessorKey: "firstName",
    header: t("users.columns.firstName"),
    cell: ({ row }) => <LongText>{row.getValue("firstName")}</LongText>,
  },
  {
    accessorKey: "lastName",
    header: t("users.columns.lastName"),
    cell: ({ row }) => <LongText>{row.getValue("lastName")}</LongText>,
  },
  {
    accessorKey: "email",
    header: t("users.columns.email"),
    cell: ({ row }) => <LongText>{row.getValue("email")}</LongText>,
  },
  {
    accessorKey: "isOwner",
    header: t("users.columns.owner"),
    cell: ({ row }) => {
      const isOwner = row.getValue("isOwner") as boolean;
      return isOwner ? (
        <Badge variant="default">{t("users.badges.owner")}</Badge>
      ) : (
        <Badge variant="secondary">{t("users.badges.user")}</Badge>
      );
    },
  },
  {
    accessorKey: "roles",
    header: t("users.columns.roles"),
    cell: ({ row }) => {
      const user = row.original;
      const roles = user.roles?.edges?.map((edge) => edge.node);
      if (!roles || roles.length === 0) {
        return <span className="text-muted-foreground">{t("users.badges.noRoles")}</span>;
      }
      return (
        <div className="flex flex-wrap gap-1">
          {roles.map((role) => (
            <Badge key={role.id} variant="outline">
              {role.name}
            </Badge>
          ))}
        </div>
      );
    },
  },
  {
    accessorKey: "status",
    header: t("users.columns.status"),
    cell: ({ row }) => {
      const status = row.getValue("status") as string;
      return (
        <Badge variant={status === "activated" ? "default" : "secondary"}>
          {status === "activated" ? t("users.status.activated") : t("users.status.deactivated")}
        </Badge>
      );
    },
  },
  {
    accessorKey: "createdAt",
    header: t("users.columns.createdAt"),
    cell: ({ row }) => {
      const date = new Date(row.getValue("createdAt"));
      return date.toLocaleDateString();
    },
  },
  {
    accessorKey: "updatedAt",
    header: t("users.columns.updatedAt"),
    cell: ({ row }) => {
      const date = new Date(row.getValue("updatedAt"));
      return date.toLocaleDateString();
    },
  },
  {
    id: "actions",
    cell: ({ row }) => <DataTableRowActions row={row} />,
  },
];
