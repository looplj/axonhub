"use client";

import { ColumnDef } from "@tanstack/react-table";
import { Checkbox } from "@/components/ui/checkbox";
import LongText from "@/components/long-text";
import { Badge } from "@/components/ui/badge";
import { User } from "../data/schema";
import { DataTableRowActions } from "./data-table-row-actions";

export const columns: ColumnDef<User>[] = [
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
    header: "First Name",
    cell: ({ row }) => <LongText>{row.getValue("firstName")}</LongText>,
  },
  {
    accessorKey: "lastName",
    header: "Last Name",
    cell: ({ row }) => <LongText>{row.getValue("lastName")}</LongText>,
  },
  {
    accessorKey: "email",
    header: "Email",
    cell: ({ row }) => <LongText>{row.getValue("email")}</LongText>,
  },
  {
    accessorKey: "isOwner",
    header: "Owner",
    cell: ({ row }) => {
      const isOwner = row.getValue("isOwner") as boolean;
      return isOwner ? (
        <Badge variant="default">Owner</Badge>
      ) : (
        <Badge variant="secondary">User</Badge>
      );
    },
  },
  {
    accessorKey: "roles",
    header: "Roles",
    cell: ({ row }) => {
      const user = row.original;
      const roles = user.roles?.edges?.map((edge) => edge.node);
      if (!roles || roles.length === 0) {
        return <span className="text-muted-foreground">No roles</span>;
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
    header: "状态",
    cell: ({ row }) => {
      const status = row.getValue("status") as string;
      return (
        <Badge variant={status === "activated" ? "default" : "secondary"}>
          {status === "activated" ? "已激活" : "已停用"}
        </Badge>
      );
    },
  },
  {
    accessorKey: "createdAt",
    header: "Created At",
    cell: ({ row }) => {
      const date = new Date(row.getValue("createdAt"));
      return date.toLocaleDateString();
    },
  },
  {
    accessorKey: "updatedAt",
    header: "Updated At",
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
